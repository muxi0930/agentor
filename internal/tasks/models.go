package tasks

import (
	"agentor/internal/appdata"
	"agentor/internal/workspace"
)

type Source struct {
	ChatType         string `json:"chatType,omitempty"` // 单聊或群聊
	Type             string `json:"type"`               // 来源类型
	AppID            string `json:"appId"`              // 来源应用
	ChatID           string `json:"chatId"`             // 来源会话
	SenderID         string `json:"senderId"`           // 原始发送者
	MessageID        string `json:"messageId"`          // 上游实际消息标识
	ReplyToMessageID string `json:"replyToMessageId"`   // 被引用的上游消息
}

type DispatchInput struct {
	AttachmentRequestID string `json:"attachmentRequestId"` // 附件原始请求，独立于完成汇报请求
	PipelineRunID       string `json:"pipelineRunId"`       // 所属流水线运行
	StepID              string `json:"stepId"`              // 目标工作区步骤
	Instruction         string `json:"instruction"`         // 本次执行指令
	CompletionCriteria  string `json:"completionCriteria"`  // 完成标准
	Model               string `json:"model"`               // 使用的模型
	Effort              string `json:"effort"`              // 推理强度
	Source              Source `json:"source"`              // 原始请求来源
	RequestID           string `json:"requestId"`           // 所属控制中心请求
	TimeoutSeconds      int    `json:"timeoutSeconds"`      // 执行超时秒数，零表示不限时
}

type Execution struct {
	AttachmentRequestID string                `json:"attachmentRequestId"` // 附件原始请求，独立于完成汇报请求
	PipelineRunID       string                `json:"pipelineRunId"`       // 所属流水线运行
	ID                  string                `json:"id"`                  // 稳定标识
	StepID              string                `json:"stepId"`              // 目标工作区步骤
	ProjectID           string                `json:"projectId"`           // 所属项目
	FeatureID           string                `json:"featureId"`           // 所属功能
	Instruction         string                `json:"instruction"`         // 本次执行指令
	CompletionCriteria  string                `json:"completionCriteria"`  // 完成标准
	Model               string                `json:"model"`               // 使用的模型
	Effort              string                `json:"effort"`              // 推理强度
	Permission          string                `json:"permission"`          // 执行权限
	Source              Source                `json:"source"`              // 原始请求来源
	RequestID           string                `json:"requestId"`           // 所属控制中心请求
	Context             workspace.StepContext `json:"context"`             // 派发时冻结的步骤上下文
	Status              string                `json:"status"`              // 处理状态
	ThreadID            string                `json:"threadId"`            // 执行会话
	TurnID              string                `json:"turnId"`              // 执行轮次
	Result              string                `json:"result"`              // 执行结果正文
	Error               string                `json:"error"`               // 失败原因
	CreatedAt           string                `json:"createdAt"`           // 创建时间
	StartedAt           string                `json:"startedAt"`           // 开始时间
	CompletedAt         string                `json:"completedAt"`         // 结束时间
	StopRequested       bool                  `json:"stopRequested"`       // 用户是否已请求停止
	TimeoutSeconds      int                   `json:"timeoutSeconds"`      // 执行超时秒数，零表示不限时
}

type NotificationPart struct {
	MessageID   string `json:"messageId"`   // 上游实际消息标识
	UploadKey   string `json:"uploadKey"`   // 已上传资源标识
	AttemptedAt string `json:"attemptedAt"` // 实际尝试发送的时间
	PayloadJSON string `json:"payloadJSON"` // 官方卡片消息内容
	ID          string `json:"id"`          // 稳定标识
	Kind        string `json:"kind"`        // 业务类型
	Channel     string `json:"channel"`     // 发送渠道
	Target      string `json:"target"`      // 发送目标
	Status      string `json:"status"`      // 处理状态
	Error       string `json:"error"`       // 失败原因
	Content     string `json:"content"`     // 发送正文
	ArtifactID  string `json:"artifactId"`  // 关联产物
}

type Notification struct {
	PipelineSessionID string             `json:"pipelineSessionId"` // 所属流水线专用会话请求
	PipelineRunID     string             `json:"pipelineRunId"`     // 所属流水线运行
	ProjectID         string             `json:"projectId"`         // 所属项目
	Source            Source             `json:"source"`            // 原始请求来源
	ID                string             `json:"id"`                // 稳定标识
	Title             string             `json:"title"`             // 显示标题
	Body              string             `json:"body"`              // 通知正文
	ExecutionID       string             `json:"executionId"`       // 所属步骤执行
	RequestID         string             `json:"requestId"`         // 所属控制中心请求
	CreatedAt         string             `json:"createdAt"`         // 创建时间
	Read              bool               `json:"read"`              // 是否已读
	Parts             []NotificationPart `json:"parts"`             // 各渠道的分项发送记录
}

type State struct {
	ControllerInstructions string            `json:"controllerInstructions"` // 当前控制中心上下文使用的完整提示词
	Executions             []Execution       `json:"executions"`             // 步骤执行记录
	Notifications          []Notification    `json:"notifications"`          // 通知记录
	Paused                 bool              `json:"paused"`                 // 是否暂停自动出队
	Requests               []Request         `json:"requests"`               // 控制中心请求记录
	ControllerThreadID     string            `json:"controllerThreadId"`     // 控制中心专用会话
	LastSteps              map[string]string `json:"lastSteps"`              // 各来源最近派发的步骤
}

type ControllerInput struct {
	Text      string `json:"text"`      // 请求正文
	ProjectID string `json:"projectId"` // 所属项目
	StepID    string `json:"stepId"`    // 目标工作区步骤
	Model     string `json:"model"`     // 使用的模型
	Effort    string `json:"effort"`    // 推理强度
	Source    Source `json:"source"`    // 原始请求来源
}

type Request struct {
	Executor     appdata.ControllerSettings `json:"executor"`     // 入队时冻结并解析默认值的控制中心配置
	ID           string                     `json:"id"`           // 稳定标识
	Kind         string                     `json:"kind"`         // 业务类型
	Text         string                     `json:"text"`         // 请求正文
	ProjectID    string                     `json:"projectId"`    // 所属项目
	StepID       string                     `json:"stepId"`       // 目标工作区步骤
	Model        string                     `json:"model"`        // 使用的模型
	Effort       string                     `json:"effort"`       // 推理强度
	Source       Source                     `json:"source"`       // 原始请求来源
	Status       string                     `json:"status"`       // 处理状态
	ThreadID     string                     `json:"threadId"`     // 执行会话
	TurnID       string                     `json:"turnId"`       // 执行轮次
	Result       string                     `json:"result"`       // 执行结果正文
	Error        string                     `json:"error"`        // 失败原因
	CreatedAt    string                     `json:"createdAt"`    // 创建时间
	CompletedAt  string                     `json:"completedAt"`  // 结束时间
	CompletionID string                     `json:"completionId"` // 待汇报的完成事件标识
	NeedsTarget  bool                       `json:"needsTarget"`  // 是否需要用户明确执行目标
	LockedTarget bool                       `json:"lockedTarget"` // 是否限制为请求已指定的步骤
}

type Scope struct {
	Role        string // 宿主工具角色
	ProjectID   string // 所属项目
	FeatureID   string // 所属功能
	StepID      string // 目标工作区步骤
	ExecutionID string // 所属步骤执行
	RequestID   string // 所属控制中心请求
	Directory   string // 当前项目目录
}
