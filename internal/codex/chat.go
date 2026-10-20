package codex

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type ReasoningOption struct {
	ReasoningEffort string `json:"reasoningEffort"`
	Description     string `json:"description"`
}

type Model struct {
	ID                        string            `json:"id"`
	Model                     string            `json:"model"`
	DisplayName               string            `json:"displayName"`
	Description               string            `json:"description"`
	IsDefault                 bool              `json:"isDefault"`
	Hidden                    bool              `json:"hidden"`
	DefaultReasoningEffort    string            `json:"defaultReasoningEffort"`
	SupportedReasoningEfforts []ReasoningOption `json:"supportedReasoningEfforts"`
	InputModalities           []string          `json:"inputModalities"`
}

func (s *Service) GetModels() ([]Model, error) {
	models := []Model{}
	cursor := ""
	for {
		var result struct {
			Data       []Model `json:"data"`
			NextCursor *string `json:"nextCursor"`
		}
		params := map[string]any{"limit": 100}
		if cursor != "" {
			params["cursor"] = cursor
		}
		if err := s.call("model/list", params, &result); err != nil {
			return nil, err
		}
		for _, model := range result.Data {
			if !model.Hidden {
				models = append(models, model)
			}
		}
		if result.NextCursor == nil {
			return models, nil
		}
		cursor = *result.NextCursor
	}
}

type ThreadStatus struct {
	Type string `json:"type"`
}

type Thread struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Preview    string       `json:"preview"`
	Cwd        string       `json:"cwd"`
	Model      string       `json:"model"`
	Originator string       `json:"originator"`
	CreatedAt  int64        `json:"createdAt"`
	UpdatedAt  int64        `json:"updatedAt"`
	Status     ThreadStatus `json:"status"`
}

type ThreadPage struct {
	Data       []Thread `json:"data"`
	NextCursor *string  `json:"nextCursor"`
}

func (s *Service) ListThreads(cursor string) (ThreadPage, error) {
	var result ThreadPage
	params := map[string]any{"limit": 100, "sortKey": "updated_at"}
	settings := s.data.GetSettings()
	if settings.ReuseCLI {
		// Sharing CLI authentication must not expose unrelated CLI conversations.
		directories := make(map[string]bool)
		for _, cwd := range settings.ChatThreads {
			directories[cwd] = true
		}
		if settings.LastThreadID != "" && settings.ProjectDirectory != "" {
			directories[settings.ProjectDirectory] = true
		}
		if len(directories) == 0 {
			return ThreadPage{Data: []Thread{}}, nil
		}
		paths := make([]string, 0, len(directories))
		for cwd := range directories {
			paths = append(paths, cwd)
		}
		params["cwd"] = paths
	}
	if cursor != "" {
		params["cursor"] = cursor
	}
	err := s.call("thread/list", params, &result)
	if settings.ReuseCLI {
		owned := make([]Thread, 0, len(result.Data))
		for _, thread := range result.Data {
			if _, ok := settings.ChatThreads[thread.ID]; ok || thread.ID == settings.LastThreadID {
				owned = append(owned, thread)
			}
		}
		result.Data = owned
	}
	if result.Data == nil {
		result.Data = []Thread{}
	}
	return result, err
}

func (s *Service) CreateThread(cwd, model string) (Thread, error) {
	if cwd == "" {
		return Thread{}, errors.New("请先选择项目目录")
	}
	info, err := os.Stat(cwd)
	if err != nil {
		return Thread{}, err
	}
	if !info.IsDir() {
		return Thread{}, errors.New("项目目录无效")
	}
	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return Thread{}, err
	}
	params := map[string]any{"cwd": cwd, "sandbox": "read-only", "approvalPolicy": "on-request", "historyMode": "paginated"}
	if model != "" {
		params["model"] = model
	}
	var result struct {
		Thread Thread `json:"thread"`
	}
	err = s.call("thread/start", params, &result)
	if err == nil {
		s.mu.Lock()
		s.opened[result.Thread.ID] = true
		s.mu.Unlock()
		_, err = s.data.RememberThread(result.Thread.ID, result.Thread.Cwd)
	}
	return result.Thread, err
}

type Input struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Path string `json:"path,omitempty"`
}

type Item struct {
	ID               string   `json:"id"`
	ClientID         string   `json:"clientId"`
	Type             string   `json:"type"`
	Text             string   `json:"text"`
	Phase            string   `json:"phase"`
	Content          []Input  `json:"content"`
	Command          string   `json:"command"`
	AggregatedOutput string   `json:"aggregatedOutput"`
	Status           string   `json:"status"`
	ExitCode         *int     `json:"exitCode"`
	Summary          []string `json:"summary"`
	ReasoningContent []string `json:"reasoningContent"`
}

func (item *Item) UnmarshalJSON(data []byte) error {
	type plain Item
	payload := struct {
		*plain
		Content json.RawMessage `json:"content"`
	}{plain: (*plain)(item)}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	switch item.Type {
	case "userMessage":
		if len(payload.Content) > 0 {
			return json.Unmarshal(payload.Content, &item.Content)
		}
	case "reasoning":
		if len(payload.Content) > 0 {
			return json.Unmarshal(payload.Content, &item.ReasoningContent)
		}
	}
	return nil
}

type TurnError struct {
	Message string `json:"message"`
}

type Turn struct {
	ID     string     `json:"id"`
	Status string     `json:"status"`
	Items  []Item     `json:"items"`
	Error  *TurnError `json:"error"`
}

type TurnPage struct {
	Data       []Turn  `json:"data"`
	NextCursor *string `json:"nextCursor"`
}

type Conversation struct {
	Thread       Thread   `json:"thread"`
	Turns        TurnPage `json:"turns"`
	ActiveTurnID string   `json:"activeTurnId"`
}

func (s *Service) OpenThread(threadID string) (Conversation, error) {
	var result Conversation
	s.mu.Lock()
	opened := s.opened[threadID]
	s.mu.Unlock()
	method := "thread/read"
	params := map[string]any{"threadId": threadID}
	if !opened {
		method = "thread/resume"
		params["excludeTurns"] = true
	}
	if err := s.call(method, params, &result); err != nil {
		return result, err
	}
	s.mu.Lock()
	s.opened[threadID] = true
	s.mu.Unlock()
	turns, err := s.ListTurns(threadID, "")
	result.Turns = turns
	if err == nil {
		if _, known := s.data.GetSettings().ChatThreads[threadID]; !known {
			_, err = s.data.RememberThread(threadID, result.Thread.Cwd)
		}
	}
	s.mu.Lock()
	result.ActiveTurnID = s.active[threadID]
	s.mu.Unlock()
	return result, err
}

func (s *Service) ListTurns(threadID, cursor string) (TurnPage, error) {
	var result TurnPage
	params := map[string]any{"threadId": threadID, "limit": 20, "itemsView": "full", "sortDirection": "desc"}
	if cursor != "" {
		params["cursor"] = cursor
	}
	err := s.call("thread/turns/list", params, &result)
	if result.Data == nil {
		result.Data = []Turn{}
	}
	return result, err
}

func (s *Service) StartTurn(threadID, text, model, effort, clientMessageID string, attachmentIDs []string) (Turn, error) {
	if strings.TrimSpace(text) == "" && len(attachmentIDs) == 0 {
		return Turn{}, errors.New("请输入消息")
	}
	input, additionalContext, err := s.buildInput(threadID, text, attachmentIDs)
	if err != nil {
		return Turn{}, err
	}
	s.mu.Lock()
	if s.status.Phase != "ready" {
		s.mu.Unlock()
		return Turn{}, errors.New("Codex 尚未就绪")
	}
	if s.active[threadID] != "" {
		s.mu.Unlock()
		return Turn{}, errors.New("当前会话正在运行，请追加指令或等待结束")
	}
	s.active[threadID] = "starting"
	s.mu.Unlock()
	params := map[string]any{"threadId": threadID, "input": input, "clientUserMessageId": clientMessageID}
	if len(additionalContext) > 0 {
		params["additionalContext"] = additionalContext
	}
	if model != "" {
		params["model"] = model
	}
	if effort != "" {
		params["effort"] = effort
	}
	var result struct {
		Turn Turn `json:"turn"`
	}
	err = s.call("turn/start", params, &result)
	s.mu.Lock()
	if s.active[threadID] == "starting" {
		if err == nil {
			s.active[threadID] = result.Turn.ID
		} else {
			delete(s.active, threadID)
		}
	}
	s.mu.Unlock()
	if err == nil && len(attachmentIDs) > 0 {
		err = s.files.BindTurn(threadID, result.Turn.ID, clientMessageID, attachmentIDs)
	}
	return result.Turn, err
}

func (s *Service) SteerTurn(threadID, text, clientMessageID string, attachmentIDs []string) error {
	if strings.TrimSpace(text) == "" && len(attachmentIDs) == 0 {
		return errors.New("请输入追加指令")
	}
	input, additionalContext, err := s.buildInput(threadID, text, attachmentIDs)
	if err != nil {
		return err
	}
	s.mu.Lock()
	turnID := s.active[threadID]
	s.mu.Unlock()
	if turnID == "" || turnID == "starting" {
		return errors.New("当前会话没有可追加的活动轮次")
	}
	params := map[string]any{"threadId": threadID, "expectedTurnId": turnID, "input": input, "clientUserMessageId": clientMessageID}
	if len(additionalContext) > 0 {
		params["additionalContext"] = additionalContext
	}
	if err := s.call("turn/steer", params, nil); err != nil {
		return err
	}
	if len(attachmentIDs) > 0 {
		return s.files.BindTurn(threadID, turnID, clientMessageID, attachmentIDs)
	}
	return nil
}

func (s *Service) buildInput(threadID, text string, attachmentIDs []string) ([]Input, map[string]map[string]string, error) {
	input := []Input{}
	if strings.TrimSpace(text) != "" {
		input = append(input, Input{Type: "text", Text: text})
	}
	context := make(map[string]map[string]string)
	if len(attachmentIDs) == 0 {
		return input, context, nil
	}
	if s.files == nil {
		return nil, nil, errors.New("附件服务不可用")
	}
	files, err := s.files.ResolveInputs(threadID, attachmentIDs)
	if err != nil {
		return nil, nil, err
	}
	for _, file := range files {
		if strings.HasPrefix(file.MediaType, "image/") {
			input = append(input, Input{Type: "localImage", Path: file.LocalPath})
		} else {
			info, _ := json.Marshal(map[string]any{"name": file.Name, "path": file.LocalPath, "mediaType": file.MediaType, "size": file.Size})
			context["agentor/attachment/"+file.ID] = map[string]string{"kind": "untrusted", "value": "用户选定的本地附件，可按需读取：" + string(info)}
		}
	}
	if len(input) == 0 {
		input = append(input, Input{Type: "text", Text: "请查看所附文件。"})
	}
	return input, context, nil
}

func (s *Service) StopTurn(threadID string) error {
	s.mu.Lock()
	turnID := s.active[threadID]
	s.mu.Unlock()
	if turnID == "" || turnID == "starting" {
		return errors.New("当前会话没有可停止的活动轮次")
	}
	return s.call("turn/interrupt", map[string]string{"threadId": threadID, "turnId": turnID}, nil)
}

func (s *Service) RenameThread(threadID, name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("请输入会话标题")
	}
	return s.call("thread/name/set", map[string]string{"threadId": threadID, "name": strings.TrimSpace(name)}, nil)
}

func (s *Service) ArchiveThread(threadID string) error {
	s.mu.Lock()
	active := s.active[threadID] != ""
	s.mu.Unlock()
	if active {
		return errors.New("请先停止运行中的会话")
	}
	return s.call("thread/archive", map[string]string{"threadId": threadID}, nil)
}

func (s *Service) ArchiveProject(cwd string) (int, error) {
	var threads []Thread
	cursor := ""
	for {
		page, err := s.ListThreads(cursor)
		if err != nil {
			return 0, err
		}
		for _, thread := range page.Data {
			if thread.Cwd == cwd {
				threads = append(threads, thread)
			}
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	s.mu.Lock()
	for _, thread := range threads {
		if s.active[thread.ID] != "" {
			s.mu.Unlock()
			return 0, errors.New("项目中仍有活动任务，请先停止后再归档")
		}
	}
	s.mu.Unlock()
	for index, thread := range threads {
		if err := s.ArchiveThread(thread.ID); err != nil {
			return index, err
		}
	}
	return len(threads), nil
}

func (s *Service) trackNotification(generation uint64, message packet) {
	if message.Method == "serverRequest/resolved" {
		var params struct {
			RequestID json.RawMessage `json:"requestId"`
			ThreadID  string          `json:"threadId"`
		}
		if json.Unmarshal(message.Params, &params) != nil {
			return
		}
		s.mu.Lock()
		if approval, ok := s.approvals[string(params.RequestID)]; ok && approval.Generation == generation && approval.ThreadID == params.ThreadID && approval.Status == "pending" {
			approval.Status = "expired"
			s.approvals[approval.ID] = approval
		}
		s.mu.Unlock()
		return
	}
	var params struct {
		ThreadID string `json:"threadId"`
		Turn     Turn   `json:"turn"`
	}
	if message.Method != "turn/started" && message.Method != "turn/completed" {
		return
	}
	if json.Unmarshal(message.Params, &params) != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status.Generation != generation {
		return
	}
	if message.Method == "turn/started" {
		s.active[params.ThreadID] = params.Turn.ID
	}
	if message.Method == "turn/completed" && s.active[params.ThreadID] == params.Turn.ID {
		delete(s.active, params.ThreadID)
	}
	if message.Method == "turn/completed" {
		s.expireApprovalsLocked(params.ThreadID, params.Turn.ID)
	}
}
