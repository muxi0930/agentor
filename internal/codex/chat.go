package codex

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type ReasoningOption struct {
	ReasoningEffort string `json:"reasoningEffort"` // 支持的思考强度
	Description     string `json:"description"`     // 说明文字
}

type Model struct {
	ID                        string            `json:"id"`                        // 模型目录标识
	Model                     string            `json:"model"`                     // 模型标识
	DisplayName               string            `json:"displayName"`               // 模型显示名称
	Description               string            `json:"description"`               // 说明文字
	IsDefault                 bool              `json:"isDefault"`                 // 是否为默认模型
	Hidden                    bool              `json:"hidden"`                    // 是否在模型列表中隐藏
	DefaultReasoningEffort    string            `json:"defaultReasoningEffort"`    // 模型默认思考强度
	SupportedReasoningEfforts []ReasoningOption `json:"supportedReasoningEfforts"` // 模型支持的思考强度
	InputModalities           []string          `json:"inputModalities"`           // 模型支持的输入类型
}

type ThreadStatus struct {
	Type string `json:"type"` // 会话状态类型
}

type Thread struct {
	ID         string       `json:"id"`         // 会话标识
	Name       string       `json:"name"`       // 会话标题
	Preview    string       `json:"preview"`    // 会话预览文字
	Cwd        string       `json:"cwd"`        // 执行目录
	Model      string       `json:"model"`      // 模型标识
	Originator string       `json:"originator"` // 会话创建来源标识
	CreatedAt  int64        `json:"createdAt"`  // 创建时间
	UpdatedAt  int64        `json:"updatedAt"`  // 最后更新时间
	Status     ThreadStatus `json:"status"`     // 当前处理状态
}

type ThreadPage struct {
	Data       []Thread `json:"data"`       // 当前分页数据
	NextCursor *string  `json:"nextCursor"` // 下一页游标
}

type Input struct {
	Type string `json:"type"`           // 输入类型
	Text string `json:"text,omitempty"` // 文字内容
	Path string `json:"path,omitempty"` // 本地资源路径
}

type Item struct {
	ID               string       `json:"id"`                   // 消息或过程节点标识
	ClientID         string       `json:"clientId"`             // 客户端消息标识
	Type             string       `json:"type"`                 // 消息或过程节点类型
	Text             string       `json:"text"`                 // 文字内容
	Phase            string       `json:"phase"`                // 助手消息阶段
	Content          []Input      `json:"content"`              // 用户输入内容
	Command          string       `json:"command"`              // 待执行命令
	AggregatedOutput string       `json:"aggregatedOutput"`     // 累计命令输出
	Status           string       `json:"status"`               // 当前处理状态
	ExitCode         *int         `json:"exitCode"`             // 命令退出码
	Summary          []string     `json:"summary"`              // 思考摘要分段
	ReasoningContent []string     `json:"reasoningContent"`     // 上游提供的思考内容分段
	DurationMs       *int64       `json:"durationMs,omitempty"` // 执行耗时毫秒数
	Changes          []FileChange `json:"changes,omitempty"`    // 文件变更明细
	Path             string       `json:"path,omitempty"`       // 本地资源路径
	Query            string       `json:"query,omitempty"`      // 搜索关键词
	Server           string       `json:"server,omitempty"`     // MCP 服务名称
	Tool             string       `json:"tool,omitempty"`       // 宿主工具名称
	Details          string       `json:"details,omitempty"`    // 补充请求详情
}

type FileChangeKind struct {
	Type     string `json:"type"`                // 文件变更类型
	MovePath string `json:"move_path,omitempty"` // 移动后的文件路径
}

type FileChange struct {
	Path string         `json:"path"` // 本地资源路径
	Kind FileChangeKind `json:"kind"` // 文件变更类型
	Diff string         `json:"diff"` // 文件变更补丁
}

type TurnError struct {
	Message string `json:"message"` // 可读提示信息
}

type Turn struct {
	ID          string     `json:"id"`                    // 轮次标识
	Status      string     `json:"status"`                // 当前处理状态
	Items       []Item     `json:"items"`                 // 本轮消息与过程节点
	Error       *TurnError `json:"error"`                 // 失败原因
	StartedAt   *int64     `json:"startedAt,omitempty"`   // 开始时间
	CompletedAt *int64     `json:"completedAt,omitempty"` // 完成时间
	DurationMs  *int64     `json:"durationMs,omitempty"`  // 执行耗时毫秒数
}

type TurnPage struct {
	Data       []Turn  `json:"data"`       // 当前分页数据
	NextCursor *string `json:"nextCursor"` // 下一页游标
}

type Conversation struct {
	Thread       Thread      `json:"thread"`               // 原生会话元信息
	Turns        TurnPage    `json:"turns"`                // 会话轮次分页
	ActiveTurnID string      `json:"activeTurnId"`         // 当前活动轮次标识
	TokenUsage   *TokenUsage `json:"tokenUsage,omitempty"` // 会话令牌用量
}

type TokenBreakdown struct {
	InputTokens           int64 `json:"inputTokens"`           // 输入令牌数
	OutputTokens          int64 `json:"outputTokens"`          // 输出令牌数
	CachedInputTokens     int64 `json:"cachedInputTokens"`     // 缓存输入令牌数
	CacheWriteInputTokens int64 `json:"cacheWriteInputTokens"` // 写入缓存的输入令牌数
	ReasoningOutputTokens int64 `json:"reasoningOutputTokens"` // 思考输出令牌数
	TotalTokens           int64 `json:"totalTokens"`           // 令牌总数
}

type TokenUsage struct {
	Total              TokenBreakdown `json:"total"`              // 累计令牌用量
	Last               TokenBreakdown `json:"last"`               // 最近一轮令牌用量
	ModelContextWindow *int64         `json:"modelContextWindow"` // 模型上下文窗口大小
}

// GetTokenUsage 读取会话最近一次上报的令牌用量快照
//
//wails:ignore
func (s *Service) GetTokenUsage(threadID string) *TokenUsage {
	s.mu.Lock()
	defer s.mu.Unlock()

	usage := s.usage[threadID]
	if usage == nil {
		return nil
	}
	result := *usage
	if usage.ModelContextWindow != nil {
		window := *usage.ModelContextWindow
		result.ModelContextWindow = &window
	}
	return &result
}

// GetModels 读取原生模型目录并补充当前 API 配置的自定义模型
func (s *Service) GetModels() ([]Model, error) {
	models := []Model{}
	cursor := ""
	for {
		var result struct {
			Data       []Model `json:"data"`       // 当前分页数据
			NextCursor *string `json:"nextCursor"` // 下一页游标
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
			settings := s.data.GetSettings()
			if !settings.ReuseCLI && settings.Model != "" {
				stored, err := s.profile(settings.ActiveAccountID)
				found := false
				for _, model := range models {
					found = found || model.Model == settings.Model
				}

				if err == nil && stored.Profile.Type == "api" && !found {
					models = append(models, Model{
						ID:                        settings.Model,
						Model:                     settings.Model,
						DisplayName:               settings.Model + "（自定义）",
						Description:               "由当前 API 账号配置，实际可用性以请求结果为准",
						InputModalities:           []string{"text"},
						SupportedReasoningEfforts: []ReasoningOption{},
					})
				}
			}
			return models, nil
		}

		cursor = *result.NextCursor
	}
}

// ListThreads 读取本应用可见的会话列表
func (s *Service) ListThreads(cursor string) (ThreadPage, error) {
	var result ThreadPage
	params := map[string]any{"limit": 100, "sortKey": "updated_at"}
	settings := s.data.GetSettings()
	if !settings.ReadExternalThreads {
		// 未开启外部会话时，仅查询本应用创建的会话所在目录
		directories := make(map[string]bool)
		for id, cwd := range settings.ChatThreads {
			if !settings.ExternalThreads[id] {
				directories[cwd] = true
			}
		}

		if settings.LastThreadID != "" && !settings.ExternalThreads[settings.LastThreadID] && settings.ProjectDirectory != "" {
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
	if !settings.ReadExternalThreads {
		owned := make([]Thread, 0, len(result.Data))
		for _, thread := range result.Data {
			if _, ok := settings.ChatThreads[thread.ID]; !settings.ExternalThreads[thread.ID] && (ok || thread.ID == settings.LastThreadID) {
				owned = append(owned, thread)
			}
		}

		result.Data = owned
	}
	if result.Data == nil {
		result.Data = []Thread{}
	}
	regular := make([]Thread, 0, len(result.Data))
	for _, thread := range result.Data {
		if settings.ManagedThreads[thread.ID] == "" {
			regular = append(regular, thread)
		}
	}

	result.Data = regular
	return result, err
}

// CreateThread 在当前项目创建普通对话
func (s *Service) CreateThread(cwd, model string) (Thread, error) {
	return s.createThread(cwd, model, s.data.EffectivePermission("", cwd), "", "", nil)
}

// CreateToolThread 创建带角色指令和宿主工具的业务线程
//
//wails:ignore
func (s *Service) CreateToolThread(cwd, model, permission, role, instructions string, tools []DynamicToolSpec) (Thread, error) {
	return s.createThread(cwd, model, permission, role, instructions, tools)
}

// UnmarshalJSON 按当前协议读取文字与思考消息的不同字段
func (item *Item) UnmarshalJSON(data []byte) error {
	type plain Item
	payload := struct {
		*plain                  // 待解码的消息公共字段
		Content json.RawMessage `json:"content"` // MCP 问答提交内容
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
	default:
		if item.Type != "agentMessage" && item.Type != "commandExecution" && item.Type != "fileChange" {
			item.Details = string(data)
		}
	}

	return nil
}

// OpenThread 恢复线程并读取分页历史
func (s *Service) OpenThread(threadID string) (Conversation, error) {
	var result Conversation
	s.mu.Lock()
	opened := s.opened[threadID]
	s.mu.Unlock()
	method := "thread/read"
	params := map[string]any{"threadId": threadID}
	if !opened {
		method = "thread/resume"
		var err error
		params, err = s.resumeParameters(threadID)
		if err != nil {
			return result, err
		}
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
			err = s.data.RememberExternalThread(threadID, result.Thread.Cwd)
		}
	}
	s.mu.Lock()
	result.ActiveTurnID = s.active[threadID]
	result.TokenUsage = s.usage[threadID]
	s.mu.Unlock()
	return result, err
}

// ListTurns 分页读取当前线程的轮次
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

// StartTurn 按会话实际权限发起一轮普通对话
func (s *Service) StartTurn(threadID, text, model, effort, clientMessageID string, attachmentIDs []string) (Turn, error) {
	settings := s.data.GetSettings()
	cwd := settings.ChatThreads[threadID]
	return s.startTurn(threadID, text, model, effort, clientMessageID, attachmentIDs, s.data.EffectivePermission(threadID, cwd), cwd)
}

// StartManagedTurn 按正式任务冻结的权限发起一轮对话
//
//wails:ignore
func (s *Service) StartManagedTurn(threadID, text, model, effort, permission, cwd, clientMessageID string, attachmentIDs []string) (Turn, error) {
	return s.startTurn(threadID, text, model, effort, clientMessageID, attachmentIDs, permission, cwd)
}

// SteerTurn 向原活动轮次追加输入
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

// StopTurn 中断当前线程的活动轮次
func (s *Service) StopTurn(threadID string) error {
	s.mu.Lock()
	turnID := s.active[threadID]
	s.mu.Unlock()
	if turnID == "" || turnID == "starting" {
		return errors.New("当前会话没有可停止的活动轮次")
	}

	return s.call("turn/interrupt", map[string]string{"threadId": threadID, "turnId": turnID}, nil)
}

// IsThreadActive 检查指定线程是否有活动轮次
//
//wails:ignore
func (s *Service) IsThreadActive(threadID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.active[threadID] != ""
}

// RenameThread 更新上游会话标题
func (s *Service) RenameThread(threadID, name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("请输入会话标题")
	}

	return s.call("thread/name/set", map[string]string{"threadId": threadID, "name": strings.TrimSpace(name)}, nil)
}

// ArchiveThread 归档上游会话
func (s *Service) ArchiveThread(threadID string) error {
	s.mu.Lock()
	active := s.active[threadID] != ""
	s.mu.Unlock()
	if active {
		return errors.New("请先停止运行中的会话")
	}

	return s.call("thread/archive", map[string]string{"threadId": threadID}, nil)
}

// ArchiveProject 归档当前目录下本应用的普通对话
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

// createThread 创建原生线程并保存本应用归属
func (s *Service) createThread(cwd, model, permission, role, instructions string, tools []DynamicToolSpec) (Thread, error) {
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

	params := map[string]any{"cwd": cwd, "sandbox": permission, "approvalPolicy": "on-request", "approvalsReviewer": "user", "historyMode": "paginated"}
	if role != "" && role != "step" {
		params["approvalPolicy"] = "never"
	}
	if len(tools) > 0 {
		params["dynamicTools"] = tools
	}
	if instructions != "" {
		params["developerInstructions"] = instructions
	}
	if model != "" {
		params["model"] = model
	}
	var result struct {
		Thread Thread `json:"thread"` // 原生会话元信息
	}
	err = s.call("thread/start", params, &result)
	if err == nil {
		s.mu.Lock()
		s.opened[result.Thread.ID] = true
		s.mu.Unlock()
		if role != "" {
			err = s.data.RememberManagedThread(result.Thread.ID, result.Thread.Cwd, role)
		} else {
			_, err = s.data.RememberThread(result.Thread.ID, result.Thread.Cwd)
		}
	}
	return result.Thread, err
}

// startTurn 登记活动状态并提交真实模型输入
func (s *Service) startTurn(threadID, text, model, effort, clientMessageID string, attachmentIDs []string, permission, cwd string) (Turn, error) {
	if strings.TrimSpace(text) == "" && len(attachmentIDs) == 0 {
		return Turn{}, errors.New("请输入消息")
	}

	input, additionalContext, err := s.buildInput(threadID, text, attachmentIDs)
	if err != nil {
		return Turn{}, err
	}

	s.mu.Lock()
	if s.closing || s.status.Phase != "ready" {
		s.mu.Unlock()
		return Turn{}, errors.New("Codex 尚未就绪")
	}
	if s.active[threadID] != "" {
		s.mu.Unlock()
		return Turn{}, errors.New("当前会话正在运行，请追加指令或等待结束")
	}

	s.turnRequests.Add(1)
	defer s.turnRequests.Done()

	s.active[threadID] = "starting"
	if s.activeText == nil {
		s.activeText = make(map[string]string)
	}
	s.activeText[threadID] = text
	s.mu.Unlock()
	params := map[string]any{"threadId": threadID, "input": input, "clientUserMessageId": clientMessageID}
	params["sandboxPolicy"] = sandboxPolicy(permission, cwd)
	params["cwd"] = cwd
	params["approvalPolicy"], params["approvalsReviewer"] = "on-request", "user"
	if role := s.data.GetSettings().ManagedThreads[threadID]; role != "" && role != "step" {
		params["approvalPolicy"] = "never"
	}
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
		Turn Turn `json:"turn"` // 上游轮次内容
	}
	err = s.call("turn/start", params, &result)
	s.mu.Lock()
	if s.active[threadID] == "starting" {
		if err == nil {
			s.active[threadID] = result.Turn.ID
		} else {
			delete(s.active, threadID)
			delete(s.activeText, threadID)
		}
	}
	closing := s.closing
	s.mu.Unlock()
	if closing && err == nil {
		s.StopTurn(threadID)
		return Turn{}, errors.New("应用正在退出，轮次已中断")
	}
	if err == nil && len(attachmentIDs) > 0 {
		err = s.files.BindTurn(threadID, result.Turn.ID, clientMessageID, attachmentIDs)
	}
	return result.Turn, err
}

// buildInput 将文字与所属附件转为当前原生输入协议
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

// trackNotification 合并当前连接的线程状态与用量事件
func (s *Service) trackNotification(generation uint64, message packet) {
	if message.Method == "item/started" || message.Method == "item/completed" {
		var params struct {
			ThreadID string `json:"threadId"` // 所属会话标识
			Item     Item   `json:"item"`     // 上游过程节点
		}
		if json.Unmarshal(message.Params, &params) != nil || params.Item.Type != "fileChange" {
			return
		}

		s.mu.Lock()
		if s.status.Generation == generation {
			if s.pendingChanges[params.ThreadID] == nil {
				s.pendingChanges[params.ThreadID] = make(map[string][]FileChange)
			}
			s.pendingChanges[params.ThreadID][params.Item.ID] = params.Item.Changes
			for id, approval := range s.approvals {
				if approval.ThreadID == params.ThreadID && approval.ItemID == params.Item.ID {
					approval.Changes = params.Item.Changes
					s.approvals[id] = approval
				}
			}
		}
		s.mu.Unlock()
		return
	}
	if message.Method == "thread/tokenUsage/updated" {
		var params struct {
			ThreadID   string     `json:"threadId"`   // 所属会话标识
			TokenUsage TokenUsage `json:"tokenUsage"` // 会话令牌用量
		}
		if json.Unmarshal(message.Params, &params) != nil {
			return
		}

		s.mu.Lock()
		if s.status.Generation == generation {
			s.usage[params.ThreadID] = &params.TokenUsage
		}
		s.mu.Unlock()
		return
	}
	if message.Method == "serverRequest/resolved" {
		var params struct {
			RequestID json.RawMessage `json:"requestId"` // 服务器请求标识
			ThreadID  string          `json:"threadId"`  // 所属会话标识
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
		ThreadID string `json:"threadId"` // 所属会话标识
		Turn     Turn   `json:"turn"`     // 上游轮次内容
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
		delete(s.activeText, params.ThreadID)
	}
	if message.Method == "turn/completed" {
		s.expireApprovalsLocked(params.ThreadID, params.Turn.ID)
		delete(s.pendingChanges, params.ThreadID)
	}
}
