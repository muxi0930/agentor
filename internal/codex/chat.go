package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
)

// GetTokenUsage 读取会话最近一次上报的令牌用量快照
//
//wails:ignore
func (s *Service) GetTokenUsage(threadID string) *codexmodel.TokenUsage {
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
func (s *Service) GetModels() ([]codexmodel.Model, error) {
	models := []codexmodel.Model{}
	cursor := ""
	for {
		var result struct {
			Data       []codexmodel.Model `json:"data"`       // 当前分页数据
			NextCursor *string            `json:"nextCursor"` // 下一页游标
		}
		params := map[string]any{"limit": 100}
		if cursor != "" {
			params["cursor"] = cursor
		}
		if err := s.call(constant.CodexMethodModelList, params, &result); err != nil {
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

				if err == nil && stored.Profile.Type == constant.CodexAccountTypeAPI && !found {
					models = append(models, codexmodel.Model{
						ID:                        settings.Model,
						Model:                     settings.Model,
						DisplayName:               settings.Model + constant.CodexCustomModelSuffix,
						Description:               constant.CodexCustomModelDescription,
						InputModalities:           []string{"text"},
						SupportedReasoningEfforts: []codexmodel.ReasoningOption{},
					})
				}
			}
			return models, nil
		}

		cursor = *result.NextCursor
	}
}

// ListThreads 读取本应用可见的会话列表
func (s *Service) ListThreads(cursor string) (codexmodel.ThreadPage, error) {
	var result codexmodel.ThreadPage
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
			return codexmodel.ThreadPage{Data: []codexmodel.Thread{}}, nil
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
	err := s.call(constant.CodexMethodThreadList, params, &result)
	if !settings.ReadExternalThreads {
		owned := make([]codexmodel.Thread, 0, len(result.Data))
		for _, thread := range result.Data {
			if _, ok := settings.ChatThreads[thread.ID]; !settings.ExternalThreads[thread.ID] && (ok || thread.ID == settings.LastThreadID) {
				owned = append(owned, thread)
			}
		}

		result.Data = owned
	}
	if result.Data == nil {
		result.Data = []codexmodel.Thread{}
	}
	return result, err
}

// CreateThread 在当前项目创建普通对话
func (s *Service) CreateThread(cwd, model string) (codexmodel.Thread, error) {
	return s.createThread(cwd, model, s.data.EffectivePermission("", cwd), "", "", nil)
}

// CreateToolThread 创建带角色指令和宿主工具的业务线程
//
//wails:ignore
func (s *Service) CreateToolThread(cwd, model, permission, role, instructions string, tools []codexmodel.DynamicToolSpec) (codexmodel.Thread, error) {
	return s.createThread(cwd, model, permission, role, instructions, tools)
}

// OpenThread 检查占用后加载历史，只读会话在读取后取消本连接订阅
func (s *Service) OpenThread(threadID string) (result codexmodel.Conversation, err error) {
	s.subscriptionMu.Lock()
	defer s.subscriptionMu.Unlock()

	s.mu.Lock()
	opened := s.opened[threadID]
	s.mu.Unlock()
	method := constant.CodexMethodThreadRead
	params := map[string]any{"threadId": threadID}
	if !opened {
		if err := s.makeSubscriptionRoom(threadID); err != nil {
			return result, err
		}

		method = constant.CodexMethodThreadResume
		params, err = s.resumeParameters(threadID)
		if err != nil {
			return result, err
		}
	}
	if callErr := s.call(method, params, &result); callErr != nil {
		if opened || !strings.Contains(callErr.Error(), "thread "+threadID+" already has an active writer") {
			return result, callErr
		}

		defer func() {
			unsubscribeErr := s.unsubscribeThread(threadID)
			if err == nil {
				err = unsubscribeErr
			}
		}()
		if err := s.call(constant.CodexMethodThreadRead, map[string]any{"threadId": threadID}, &result); err != nil {
			return result, err
		}
		result.ReadOnly = true
	} else if !opened {
		s.mu.Lock()
		s.opened[threadID] = true
		if s.data.GetSettings().ManagedThreads[threadID] == "" {
			s.subscriptions = append(s.subscriptions, threadID)
		}
		s.mu.Unlock()
	}

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
func (s *Service) ListTurns(threadID, cursor string) (codexmodel.TurnPage, error) {
	var result codexmodel.TurnPage
	params := map[string]any{"threadId": threadID, "limit": 20, "itemsView": "full", "sortDirection": "desc"}
	if cursor != "" {
		params["cursor"] = cursor
	}
	err := s.call(constant.CodexMethodThreadTurnsList, params, &result)
	if result.Data == nil {
		result.Data = []codexmodel.Turn{}
	}
	return result, err
}

// StartTurn 按会话实际权限发起一轮普通对话
func (s *Service) StartTurn(threadID, text, model, effort, clientMessageID string, attachmentIDs []string) (codexmodel.Turn, error) {
	settings := s.data.GetSettings()
	cwd := settings.ChatThreads[threadID]
	tier := constant.CodexServiceTierDefault
	if settings.FastMode {
		tier = constant.CodexServiceTierFast
	}
	return s.startTurn(threadID, text, model, effort, clientMessageID, attachmentIDs, s.data.EffectivePermission(threadID, cwd), cwd, tier)
}

// StartManagedTurn 按正式任务冻结的权限发起一轮对话
//
//wails:ignore
func (s *Service) StartManagedTurn(threadID, text, model, effort, permission, cwd, clientMessageID string, attachmentIDs []string, serviceTier string) (codexmodel.Turn, error) {
	return s.startTurn(threadID, text, model, effort, clientMessageID, attachmentIDs, permission, cwd, serviceTier)
}

// SteerTurn 向原活动轮次追加输入
func (s *Service) SteerTurn(threadID, text, clientMessageID string, attachmentIDs []string) error {
	if strings.TrimSpace(text) == "" && len(attachmentIDs) == 0 {
		return constant.ErrCodexSteerTextRequired
	}

	input, additionalContext, err := s.buildInput(threadID, text, attachmentIDs)
	if err != nil {
		return err
	}

	s.mu.Lock()
	turnID := s.active[threadID]
	s.mu.Unlock()
	if turnID == "" || turnID == constant.CodexPhaseStarting {
		return constant.ErrCodexNoSteerableTurn
	}

	params := map[string]any{"threadId": threadID, "expectedTurnId": turnID, "input": input, "clientUserMessageId": clientMessageID}
	if len(additionalContext) > 0 {
		params["additionalContext"] = additionalContext
	}
	if err := s.call(constant.CodexMethodTurnSteer, params, nil); err != nil {
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
	if turnID == "" || turnID == constant.CodexPhaseStarting {
		return constant.ErrCodexNoStoppableTurn
	}

	return s.call(constant.CodexMethodTurnInterrupt, map[string]string{"threadId": threadID, "turnId": turnID}, nil)
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
		return constant.ErrCodexThreadNameRequired
	}

	return s.call(constant.CodexMethodThreadNameSet, map[string]string{"threadId": threadID, "name": strings.TrimSpace(name)}, nil)
}

// ArchiveThread 归档上游会话
func (s *Service) ArchiveThread(threadID string) error {
	s.mu.Lock()
	active := s.active[threadID] != ""
	s.mu.Unlock()
	if active {
		return constant.ErrCodexThreadActive
	}

	return s.call(constant.CodexMethodThreadArchive, map[string]string{"threadId": threadID}, nil)
}

// ArchiveProject 归档当前目录下本应用的普通对话
func (s *Service) ArchiveProject(cwd string) (int, error) {
	var threads []codexmodel.Thread
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
			return 0, constant.ErrCodexProjectActive
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
func (s *Service) createThread(cwd, model, permission, role, instructions string, tools []codexmodel.DynamicToolSpec) (codexmodel.Thread, error) {
	if cwd == "" {
		return codexmodel.Thread{}, constant.ErrCodexProjectDirectoryRequired
	}

	info, err := os.Stat(cwd)
	if err != nil {
		return codexmodel.Thread{}, err
	}
	if !info.IsDir() {
		return codexmodel.Thread{}, constant.ErrCodexProjectDirectoryInvalid
	}

	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return codexmodel.Thread{}, err
	}

	s.subscriptionMu.Lock()
	defer s.subscriptionMu.Unlock()
	if role == "" {
		if err := s.makeSubscriptionRoom(""); err != nil {
			return codexmodel.Thread{}, err
		}
	}

	params := map[string]any{"cwd": cwd, "sandbox": permission, "approvalPolicy": "on-request", "approvalsReviewer": "user", "historyMode": "paginated"}
	if role != "" && role != constant.ToolRouterRoleStep {
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
		Thread codexmodel.Thread `json:"thread"` // 原生会话元信息
	}
	err = s.call(constant.CodexMethodThreadStart, params, &result)
	if err == nil {
		s.mu.Lock()
		s.opened[result.Thread.ID] = true
		if role == "" {
			s.subscriptions = append(s.subscriptions, result.Thread.ID)
		}
		s.mu.Unlock()
		if role != "" {
			err = s.data.RememberManagedThread(result.Thread.ID, result.Thread.Cwd, role)
		} else {
			_, err = s.data.RememberThread(result.Thread.ID, result.Thread.Cwd)
		}
		if err != nil {
			return result.Thread, err
		}

		prefix := ""
		switch role {
		case constant.WorkspaceKindStep:
			prefix = "[AgentorWoker]"
		case constant.ToolRouterRoleController:
			prefix = "[AgentorCtl]"
		case constant.ToolRouterRolePipelineManager, constant.ToolRouterRolePipelineConfig:
			prefix = "[AgentorPipe]"
		}
		if prefix != "" {
			name := &managedThreadName{prefix: prefix}
			s.mu.Lock()
			s.threadNames[result.Thread.ID] = name
			c := s.client
			s.mu.Unlock()
			name.mu.Lock()
			err = name.apply(s.ctx, c, &result.Thread)
			name.mu.Unlock()
			if err != nil {
				return result.Thread, err
			}
		}
	}
	return result.Thread, err
}

// startTurn 登记活动状态并提交真实模型输入
func (s *Service) startTurn(threadID, text, model, effort, clientMessageID string, attachmentIDs []string, permission, cwd, serviceTier string) (codexmodel.Turn, error) {
	if strings.TrimSpace(text) == "" && len(attachmentIDs) == 0 {
		return codexmodel.Turn{}, constant.ErrCodexMessageRequired
	}

	input, additionalContext, err := s.buildInput(threadID, text, attachmentIDs)
	if err != nil {
		return codexmodel.Turn{}, err
	}

	s.subscriptionMu.Lock()
	defer s.subscriptionMu.Unlock()

	s.mu.Lock()
	if s.closing || s.status.Phase != constant.CodexPhaseReady {
		s.mu.Unlock()
		return codexmodel.Turn{}, constant.ErrCodexNotReady
	}
	if s.active[threadID] != "" {
		s.mu.Unlock()
		return codexmodel.Turn{}, constant.ErrCodexThreadBusy
	}

	s.turnRequests.Add(1)
	defer s.turnRequests.Done()

	s.active[threadID] = constant.CodexPhaseStarting
	if s.activeText == nil {
		s.activeText = make(map[string]string)
	}
	s.activeText[threadID] = text
	s.mu.Unlock()
	if err := s.ensureOpen(threadID); err != nil {
		s.mu.Lock()
		delete(s.active, threadID)
		delete(s.activeText, threadID)
		s.mu.Unlock()
		return codexmodel.Turn{}, err
	}

	params := map[string]any{"threadId": threadID, "input": input, "clientUserMessageId": clientMessageID}
	params["sandboxPolicy"] = sandboxPolicy(permission, cwd)
	params["cwd"] = cwd
	params["approvalPolicy"], params["approvalsReviewer"] = "on-request", "user"
	if role := s.data.GetSettings().ManagedThreads[threadID]; role != "" && role != constant.ToolRouterRoleStep {
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
	if serviceTier != "" {
		params["serviceTierForTurn"] = serviceTier
	}
	var result struct {
		Turn codexmodel.Turn `json:"turn"` // 上游轮次内容
	}
	err = s.call(constant.CodexMethodTurnStart, params, &result)
	s.mu.Lock()
	if s.active[threadID] == constant.CodexPhaseStarting {
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
		return codexmodel.Turn{}, constant.ErrCodexTurnInterrupted
	}
	if err == nil && len(attachmentIDs) > 0 {
		err = s.files.BindTurn(threadID, result.Turn.ID, clientMessageID, attachmentIDs)
	}
	return result.Turn, err
}

// buildInput 将文字与所属附件转为当前原生输入协议
func (s *Service) buildInput(threadID, text string, attachmentIDs []string) ([]codexmodel.Input, map[string]map[string]string, error) {
	input := []codexmodel.Input{}
	if strings.TrimSpace(text) != "" {
		input = append(input, codexmodel.Input{Type: "text", Text: text})
	}
	context := make(map[string]map[string]string)
	if len(attachmentIDs) == 0 {
		return input, context, nil
	}
	if s.files == nil {
		return nil, nil, constant.ErrCodexArtifactServiceUnavailable
	}

	files, err := s.files.ResolveInputs(threadID, attachmentIDs)
	if err != nil {
		return nil, nil, err
	}

	for _, file := range files {
		if strings.HasPrefix(file.MediaType, "image/") {
			input = append(input, codexmodel.Input{Type: "localImage", Path: file.LocalPath})
		} else {
			info, _ := json.Marshal(map[string]any{"name": file.Name, "path": file.LocalPath, "mediaType": file.MediaType, "size": file.Size})
			context["agentor/attachment/"+file.ID] = map[string]string{"kind": "untrusted", "value": constant.CodexAttachmentInstructions + string(info)}
		}
	}

	if len(input) == 0 {
		input = append(input, codexmodel.Input{Type: "text", Text: constant.CodexAttachmentFallbackPrompt})
	}
	return input, context, nil
}

// trackNotification 合并当前连接的线程状态与用量事件
func (s *Service) trackNotification(generation uint64, message packet) {
	if message.Method == constant.CodexMethodThreadClosed {
		var params struct {
			ThreadID string `json:"threadId"` // 已卸载的会话标识
		}
		if json.Unmarshal(message.Params, &params) != nil {
			return
		}

		s.mu.Lock()
		if s.status.Generation == generation {
			s.forgetThreadLocked(params.ThreadID)
		}
		s.mu.Unlock()
		return
	}
	s.trackManagedThreadName(generation, message)

	if message.Method == constant.CodexMethodItemStarted || message.Method == constant.CodexMethodItemCompleted {
		var params struct {
			ThreadID string          `json:"threadId"` // 所属会话标识
			Item     codexmodel.Item `json:"item"`     // 上游过程节点
		}
		if json.Unmarshal(message.Params, &params) != nil || params.Item.Type != constant.CodexItemFileChange {
			return
		}

		s.mu.Lock()
		if s.status.Generation == generation {
			if s.pendingChanges[params.ThreadID] == nil {
				s.pendingChanges[params.ThreadID] = make(map[string][]codexmodel.FileChange)
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
	if message.Method == constant.CodexMethodThreadTokenUsageUpdated {
		var params struct {
			ThreadID   string                `json:"threadId"`   // 所属会话标识
			TokenUsage codexmodel.TokenUsage `json:"tokenUsage"` // 会话令牌用量
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
		if approval, ok := s.approvals[string(params.RequestID)]; ok && approval.Generation == generation && approval.ThreadID == params.ThreadID && approval.Status == constant.CodexStatusPending {
			approval.Status = constant.CodexApprovalExpired
			s.approvals[approval.ID] = approval
		}
		s.mu.Unlock()
		return
	}

	var params struct {
		ThreadID string          `json:"threadId"` // 所属会话标识
		Turn     codexmodel.Turn `json:"turn"`     // 上游轮次内容
	}
	if message.Method != constant.CodexMethodTurnStarted && message.Method != constant.CodexMethodTurnCompleted {
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
	if message.Method == constant.CodexMethodTurnStarted {
		s.active[params.ThreadID] = params.Turn.ID
	}
	if message.Method == constant.CodexMethodTurnCompleted && s.active[params.ThreadID] == params.Turn.ID {
		delete(s.active, params.ThreadID)
		delete(s.activeText, params.ThreadID)
	}
	if message.Method == constant.CodexMethodTurnCompleted {
		s.expireApprovalsLocked(params.ThreadID, params.Turn.ID)
		delete(s.pendingChanges, params.ThreadID)
	}
}
