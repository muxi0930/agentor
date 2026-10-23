package model

import (
	tasksmodel "agentor/internal/tasks/model"
	workspacemodel "agentor/internal/workspace/model"
)

type InputField struct {
	Name     string `json:"name"`     // 输入字段名称
	Type     string `json:"type"`     // 支持的值类型
	Required bool   `json:"required"` // 是否必须提供输入
}

type Position struct {
	X float64 `json:"x"` // 画布横坐标
	Y float64 `json:"y"` // 画布纵坐标
}

type Branch struct {
	Name        string `json:"name"`        // 预先声明的业务结果标识
	Description string `json:"description"` // 分支含义与选择条件
	Target      string `json:"target"`      // 下一节点标识，空值表示未处理并终止
}

type Action struct {
	ID             string         `json:"id"`             // 图内唯一节点标识
	Kind           string         `json:"kind"`           // static、agent 或终点 end
	Type           string         `json:"type"`           // 静态动作标识或 agent.run、end.success、end.failure
	Name           string         `json:"name"`           // 节点显示名称
	Params         map[string]any `json:"params"`         // 输入数据与结构化引用
	Position       Position       `json:"position"`       // 画布位置
	Branches       []Branch       `json:"branches"`       // 业务结果到后续节点的单选路由
	Prompt         string         `json:"prompt"`         // Agent 执行提示词
	Criteria       string         `json:"criteria"`       // Agent 自检验收标准
	Permission     string         `json:"permission"`     // 只读或工作区写入权限
	MaxRetries     int            `json:"maxRetries"`     // Agent 失败后的额外尝试次数
	TimeoutSeconds int            `json:"timeoutSeconds"` // Agent 单次尝试超时秒数
	FailureTarget  string         `json:"failureTarget"`  // Agent 重试耗尽后的处理节点
	StepSlot       string         `json:"stepSlot"`       // 模板预设的项目步骤插槽
	StepID         string         `json:"stepId"`         // 实例绑定的实际步骤
}

type Task struct {
	ID         string       `json:"id"`         // 流水线或模板标识
	ProjectID  string       `json:"projectId"`  // 所属项目，独立实例与模板为空
	TemplateID string       `json:"templateId"` // 创建时使用的模板，仅记录来源
	Directory  string       `json:"directory"`  // 独立实例执行目录，空值使用本次运行目录
	Name       string       `json:"name"`       // 流水线显示名称
	Entry      string       `json:"entry"`      // 入口节点标识
	Inputs     []InputField `json:"inputs"`     // 运行输入字段
	Actions    []Action     `json:"actions"`    // 节点图，执行顺序由分支连线决定
}

type InstantiateInput struct {
	TemplateID string            `json:"templateId"` // 模板标识
	ProjectID  string            `json:"projectId"`  // 可选的项目范围
	Name       string            `json:"name"`       // 新实例名称
	Directory  string            `json:"directory"`  // 独立实例执行目录
	Bindings   map[string]string `json:"bindings"`   // 步骤插槽到步骤标识的映射
}

type NodeReport struct {
	Outcome string `json:"outcome"` // success 表示验收通过，failure 表示本次尝试失败
	Branch  string `json:"branch"`  // 通过验收后选择的已声明业务分支
	Summary string `json:"summary"` // 验收依据或失败原因
	Output  any    `json:"output"`  // 交付给后续节点的自由文本或 JSON
}

type Parameter struct {
	Name     string `json:"name"`     // 参数名称
	Type     string `json:"type"`     // 参数值类型
	Required bool   `json:"required"` // 是否为必填参数
}

type ActionSpec struct {
	Branches        []Branch    `json:"branches"`        // 动作可以返回的固定业务结果
	RequiresProject bool        `json:"requiresProject"` // 是否依赖项目范围
	Name            string      `json:"name"`            // 静态动作名称或 Agent 类型
	Description     string      `json:"description"`     // 动作行为说明
	Parameters      []Parameter `json:"parameters"`      // 动作接受的参数
	Result          string      `json:"result"`          // 动作返回值说明
}

type RunInput struct {
	TaskID    string            `json:"taskId"`    // 待运行任务标识
	Inputs    map[string]any    `json:"inputs"`    // 本次运行的输入值
	Model     string            `json:"model"`     // 本次使用的模型
	Effort    string            `json:"effort"`    // 本次使用的推理强度
	Source    tasksmodel.Source `json:"source"`    // 任务请求来源
	RequestID string            `json:"requestId"` // 关联的外部请求标识
}

type ActionResult struct {
	Kind        string         `json:"kind"`             // 节点类型
	Branch      string         `json:"branch"`           // 实际选择的业务分支
	ThreadID    string         `json:"threadId"`         // Agent 尝试使用的原生会话
	TurnID      string         `json:"turnId"`           // Agent 尝试使用的原生轮次
	Report      *NodeReport    `json:"report,omitempty"` // 工具汇报，轮次正常结束后才可推进
	ExecutionID string         `json:"executionId"`      // 可选绑定步骤的执行记录
	Attempt     int            `json:"attempt"`          // 当前节点的尝试次数，历史记录持续保留
	ID          string         `json:"id"`               // 对应动作标识
	Type        string         `json:"type"`             // 静态动作名称或 Agent 类型
	Status      string         `json:"status"`           // 动作执行状态
	Inputs      map[string]any `json:"inputs"`           // 解析引用后的实际参数
	Output      any            `json:"output"`           // 动作返回结果
	Error       string         `json:"error"`            // 动作错误说明
	StartedAt   string         `json:"startedAt"`        // 动作开始时间
	CompletedAt string         `json:"completedAt"`      // 动作结束时间
}

type Run struct {
	CurrentNode    string                                `json:"currentNode"`        // 已持久化的下一执行位置
	Directory      string                                `json:"directory"`          // 冻结的工作目录
	StepContexts   map[string]workspacemodel.StepContext `json:"stepContexts"`       // 按节点冻结的项目步骤上下文
	Worktree       *Worktree                             `json:"worktree,omitempty"` // 本次运行专属的工作区记录
	PauseRequested bool                                  `json:"pauseRequested"`     // 当前节点结束后暂停推进
	ID             string                                `json:"id"`                 // 运行记录标识
	ProjectID      string                                `json:"projectId"`          // 所属项目标识
	TaskID         string                                `json:"taskId"`             // 对应任务标识
	Task           Task                                  `json:"task"`               // 本次运行冻结的节点图定义
	Inputs         map[string]any                        `json:"inputs"`             // 本次运行冻结的输入值
	Model          string                                `json:"model"`              // 实际使用的模型
	Effort         string                                `json:"effort"`             // 实际使用的推理强度
	Source         tasksmodel.Source                     `json:"source"`             // 任务请求来源
	RequestID      string                                `json:"requestId"`          // 关联的外部请求标识
	ScheduleID     string                                `json:"scheduleId"`         // 发起运行的调度标识
	Status         string                                `json:"status"`             // 流水线运行状态
	Error          string                                `json:"error"`              // 运行错误说明
	CreatedAt      string                                `json:"createdAt"`          // 运行创建时间
	CompletedAt    string                                `json:"completedAt"`        // 运行结束时间
	Actions        []ActionResult                        `json:"actions"`            // 按执行顺序保存的动作结果
	ArtifactIDs    []string                              `json:"artifactIds"`        // 本次运行生成的报告产物标识
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

type State struct {
	Templates []Task     `json:"templates"` // 独立流水线模板
	Tasks     []Task     `json:"tasks"`     // 可运行的独立或项目流水线实例
	Schedules []Schedule `json:"schedules"` // 实例定时触发配置
	Runs      []Run      `json:"runs"`      // 冻结的运行与尝试记录
}

type Worktree struct {
	Directory    string `json:"directory"`    // 宿主分配的独立工作区目录
	Repository   string `json:"repository"`   // 来源工作区目录
	SourceBranch string `json:"sourceBranch"` // 创建时的来源分支
	SourceCommit string `json:"sourceCommit"` // 创建时的来源提交
	Branch       string `json:"branch"`       // 本次运行专属分支
	MergedCommit string `json:"mergedCommit"` // 合并成功的提交
}
