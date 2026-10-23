package constant

import (
	"time"
)

// tasks 模块的固定配置与提示文案
const (
	TasksStepInstructions              = "你正在执行 agentor 的正式步骤。知识和产物工具的范围由宿主绑定至当前步骤。使用 read_knowledge 获取绑定文档，使用 prepare_knowledge_document / sync_knowledge_document 维护文档，使用 register_artifact 登记本次产物。遵守用户的步骤指令；不要伪造工具结果。"
	TasksQueueCancelledMessage         = "用户取消排队"
	TasksContinueCommand               = "继续"
	TasksControllerTitlePrefix         = "控制中心 · "
	TasksControllerRequestTemplate     = "请求类型：%s\n请求 ID：%s\n项目范围：%s（空表示用户允许查询整个工作区）\n明确步骤目标：%s\n需要用户明确目标：%t\n\n用户请求或宿主完成事件：\n%s"
	TasksCompletionPromptTemplate      = "执行 %s 已结束。步骤：%s；状态：%s。\n结果：\n%s\n错误：%s\n只向用户汇报结果，不再次执行。"
	TasksControllerInterruptedMessage  = "应用退出时控制中心尚未结束；未自动重放"
	TasksExecutionInterruptedMessage   = "应用退出时任务尚未结束；未自动重放"
	TasksExecutionShutdownMessage      = "应用退出，任务已取消"
	TasksRequestShutdownMessage        = "应用退出，请求已取消"
	TasksExecutionFinishedMessage      = "任务已结束"
	TasksPersistFailureSuffix          = "；保存记录失败："
	TasksExecutionHeading              = "# 正式步骤执行"
	TasksProjectHeading                = "## 项目："
	TasksVersionHeading                = "## 版本："
	TasksFeatureHeading                = "## 功能："
	TasksStepHeading                   = "## 步骤："
	TasksInstructionHeading            = "## 本次执行指令"
	TasksCompletionCriteriaHeading     = "## 完成标准"
	TasksKnowledgeIndexHeading         = "## 知识文档索引（需要正文时调用 read_knowledge）"
	TasksExecutorChoiceTemplate        = "你可以在 [ %s ] 模型中选择，并在 [ %s ] 中决定 effort；"
	TasksExecutorSelectionInstructions = "\n派发步骤、运行流水线或请求专用会话时，必须通过工具的 model 和 effort 参数明确选择。空字符串表示跟随本次默认会话，但只有解析后的值在允许范围内时才可使用。仅选择模型支持的 effort；工具返回不兼容或超出范围时，重新选择允许的组合，不要重复提交相同组合。"
	TasksVersionCreationInstructions   = "\n创建项目版本时使用 create_version，显式传入本次请求范围内的 projectId；无法唯一确定目标项目时先询问用户。"
)

// tasks 模块的业务标识
const (
	TasksRequestKindInstruction     = "instruction"
	TasksRequestKindExecutionResult = "execution_result"
	TasksRequestKindPipelineResult  = "pipeline_result"
	TasksStatusQueued               = "queued"
	TasksStatusRunning              = "running"
	TasksStatusCompleted            = "completed"
	TasksStatusFailed               = "failed"
	TasksStatusInterrupted          = "interrupted"
	TasksNotificationStatusSent     = "sent"
	TasksChangedEvent               = "tasks:changed"
)

// tasks 模块的默认值与运行限制
const (
	TasksStopConfirmationTimeout = 15 * time.Second
	TasksDeleteTimeout           = 30 * time.Second
	TasksQueuePollInterval       = 100 * time.Millisecond
)

// tasks 模块的状态与协议标识
const (
	TasksStatusError     = "error"
	TasksStatusCancelled = "cancelled"
	TasksSourceDesktop   = "desktop"
	TasksStateFilename   = "tasks.json"
)
