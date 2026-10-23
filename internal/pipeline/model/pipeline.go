package model

import (
	tasksmodel "agentor/internal/tasks/model"
)

type InputField struct {
	Name     string `json:"name"`     // 输入字段名称
	Type     string `json:"type"`     // 支持的值类型
	Required bool   `json:"required"` // 是否必须提供输入
}

type Action struct {
	ID     string         `json:"id"`     // 动作标识
	Type   string         `json:"type"`   // 固定动作名称
	Params map[string]any `json:"params"` // 动作参数及完整值引用
}

type Task struct {
	ID        string       `json:"id"`        // 任务标识
	ProjectID string       `json:"projectId"` // 所属项目标识
	Name      string       `json:"name"`      // 任务显示名称
	Inputs    []InputField `json:"inputs"`    // 运行输入字段
	Actions   []Action     `json:"actions"`   // 按顺序执行的固定动作
}

type Parameter struct {
	Name     string `json:"name"`     // 参数名称
	Type     string `json:"type"`     // 参数值类型
	Required bool   `json:"required"` // 是否为必填参数
}

type ActionSpec struct {
	Name        string      `json:"name"`        // 固定动作名称
	Description string      `json:"description"` // 动作行为说明
	Parameters  []Parameter `json:"parameters"`  // 动作接受的参数
	Result      string      `json:"result"`      // 动作返回值说明
}

type RunInput struct {
	SessionRequestID string            `json:"sessionRequestId"` // 发起运行的专用会话请求标识
	TaskID           string            `json:"taskId"`           // 待运行任务标识
	Inputs           map[string]any    `json:"inputs"`           // 本次运行的输入值
	Model            string            `json:"model"`            // 本次使用的模型
	Effort           string            `json:"effort"`           // 本次使用的推理强度
	Source           tasksmodel.Source `json:"source"`           // 任务请求来源
	RequestID        string            `json:"requestId"`        // 关联的外部请求标识
}

type ActionResult struct {
	Attempt     int            `json:"attempt"`     // 当前动作的尝试次数，历史记录持续保留
	ID          string         `json:"id"`          // 对应动作标识
	Type        string         `json:"type"`        // 固定动作名称
	Status      string         `json:"status"`      // 动作执行状态
	Inputs      map[string]any `json:"inputs"`      // 解析引用后的实际参数
	Output      any            `json:"output"`      // 动作返回结果
	Error       string         `json:"error"`       // 动作错误说明
	StartedAt   string         `json:"startedAt"`   // 动作开始时间
	CompletedAt string         `json:"completedAt"` // 动作结束时间
}

type Run struct {
	Worktree         *Worktree         `json:"worktree,omitempty"` // 本次运行专属的工作区记录
	PauseRequested   bool              `json:"pauseRequested"`     // 当前节点结束后暂停推进
	SessionRequestID string            `json:"sessionRequestId"`   // 发起运行的专用会话请求标识
	ID               string            `json:"id"`                 // 运行记录标识
	ProjectID        string            `json:"projectId"`          // 所属项目标识
	TaskID           string            `json:"taskId"`             // 对应任务标识
	Task             Task              `json:"task"`               // 本次运行冻结的任务定义
	Inputs           map[string]any    `json:"inputs"`             // 本次运行冻结的输入值
	Model            string            `json:"model"`              // 实际使用的模型
	Effort           string            `json:"effort"`             // 实际使用的推理强度
	Source           tasksmodel.Source `json:"source"`             // 任务请求来源
	RequestID        string            `json:"requestId"`          // 关联的外部请求标识
	ScheduleID       string            `json:"scheduleId"`         // 发起运行的调度标识
	Status           string            `json:"status"`             // 流水线运行状态
	Error            string            `json:"error"`              // 运行错误说明
	CreatedAt        string            `json:"createdAt"`          // 运行创建时间
	CompletedAt      string            `json:"completedAt"`        // 运行结束时间
	Actions          []ActionResult    `json:"actions"`            // 按执行顺序保存的动作结果
	ArtifactIDs      []string          `json:"artifactIds"`        // 本次运行生成的报告产物标识
}

type Schedule struct {
	ID              string         `json:"id"`              // 调度标识
	ProjectID       string         `json:"projectId"`       // 所属项目标识
	TaskID          string         `json:"taskId"`          // 触发的任务标识
	Name            string         `json:"name"`            // 调度显示名称
	Kind            string         `json:"kind"`            // once 或 interval 触发类型
	Enabled         bool           `json:"enabled"`         // 是否启用调度
	At              string         `json:"at"`              // 一次性触发的 UTC 时间
	Anchor          string         `json:"anchor"`          // 间隔触发的固定锚点时间
	IntervalSeconds int64          `json:"intervalSeconds"` // 重复触发的间隔秒数
	Timezone        string         `json:"timezone"`        // 解释本地时间的 IANA 时区
	MisfirePolicy   string         `json:"misfirePolicy"`   // 错过触发时的处理策略
	Inputs          map[string]any `json:"inputs"`          // 触发任务时使用的输入值
	NextAt          string         `json:"nextAt"`          // 下次触发时间
	LastAt          string         `json:"lastAt"`          // 最近检查到期的时间
	LastOutcome     string         `json:"lastOutcome"`     // 最近触发结果说明
	LastRunID       string         `json:"lastRunId"`       // 最近创建的运行标识
}

type SessionInput struct {
	ProjectID string            `json:"projectId"` // 专用会话所属项目标识
	Role      string            `json:"role"`      // 管理或配置会话角色
	Text      string            `json:"text"`      // 用户请求或完成结果正文
	Model     string            `json:"model"`     // 请求使用的模型
	Effort    string            `json:"effort"`    // 请求使用的推理强度
	Source    tasksmodel.Source `json:"source"`    // 会话请求来源
	RequestID string            `json:"requestId"` // 关联的外部请求标识
}

type SessionRequest struct {
	Kind            string       `json:"kind"`            // 用户指令或运行完成事件类型
	CompletionRunID string       `json:"completionRunId"` // 完成事件对应的运行标识
	ID              string       `json:"id"`              // 专用会话请求标识
	Input           SessionInput `json:"input"`           // 本次请求冻结的输入参数
	Status          string       `json:"status"`          // 会话请求状态
	ThreadID        string       `json:"threadId"`        // 对应的原生会话标识
	TurnID          string       `json:"turnId"`          // 对应的原生轮次标识
	Result          string       `json:"result"`          // 会话处理结果正文
	Error           string       `json:"error"`           // 会话错误说明
	CreatedAt       string       `json:"createdAt"`       // 请求创建时间
	CompletedAt     string       `json:"completedAt"`     // 请求结束时间
}

type State struct {
	Tasks     []Task            `json:"tasks"`     // 已配置的流水线任务
	Schedules []Schedule        `json:"schedules"` // 任务调度配置
	Runs      []Run             `json:"runs"`      // 流水线运行记录
	Requests  []SessionRequest  `json:"requests"`  // 专用会话请求队列
	Threads   map[string]string `json:"threads"`   // 按项目与角色保存的会话标识
	Paused    bool              `json:"paused"`    // 专用会话队列是否暂停
}

type Worktree struct {
	Directory    string `json:"directory"`    // 宿主分配的独立工作区目录
	Repository   string `json:"repository"`   // 来源工作区目录
	SourceBranch string `json:"sourceBranch"` // 创建时的来源分支
	SourceCommit string `json:"sourceCommit"` // 创建时的来源提交
	Branch       string `json:"branch"`       // 本次运行专属分支
	MergedCommit string `json:"mergedCommit"` // 合并成功的提交
}
