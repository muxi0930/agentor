package constant

import (
	"time"
)

// pipeline 模块的固定配置与提示文案
const (
	PipelineRunInterruptedMessage      = "应用退出时流水线尚未结束，未自动重放"
	PipelineSessionInterruptedMessage  = "应用退出时会话请求尚未结束，未自动重放"
	PipelineRunShutdownMessage         = "应用退出，流水线已取消"
	PipelineSessionShutdownMessage     = "应用退出，会话请求已取消"
	PipelineQueueCancelledMessage      = "用户取消排队"
	PipelineSessionTitlePrefix         = "流水线会话 · "
	PipelineManagerInstructions        = "你是 agentor 流水线管理助手。只用宿主工具查询固定动作、任务与记录，运行已有任务，通过 control_pipeline_run 暂停、继续、重试和停止运行；Go 自动推进节点，运行定义和输入冻结，不能跳过节点或自行宣告成功；不写脚本、不修改项目文件。输入引用仅为整个参数值的 {input:字段名}，结果引用仅为 {result:已完成动作ID}。"
	PipelineConfigInstructions         = "你是 agentor 流水线配置助手。通过宿主工具保存/删除任务和调度、验证 JSON 配置；不执行任务，不写脚本或修改项目文件。先读取固定动作目录。任务由 inputs 与顺序 actions 组成，只支持整个参数值的 {input:字段名}、{result:已完成动作ID} 引用。所有对象归当前项目。"
	PipelineSessionPromptTemplate      = "项目 ID：%s\n项目名称：%s\n请求类型：%s（非 instruction 时只汇报完成结果，不重新执行）\n\n用户请求或完成事件：\n%s"
	PipelineReportHeaderTemplate       = "流水线 **%s**：%s\n\n"
	PipelineReportErrorTemplate        = "\n错误：%s\n"
	PipelineReportArtifactsTemplate    = "\n报告产物：%s\n"
	PipelineInspectActionDescription   = "读取当前项目的结构及步骤最新状态。"
	PipelineInspectActionResult        = "项目结构和步骤状态对象"
	PipelineDispatchActionDescription  = "进入正式步骤队列并等待完成。停止流水线只取消本次发起的执行。"
	PipelineDispatchActionResult       = "executionId、status、result、artifactIds 对象"
	PipelineKnowledgeActionDescription = "读取当前项目共享或指定功能绑定的知识正文。"
	PipelineKnowledgeActionResult      = "document 和 content 对象"
	PipelineReportActionDescription    = "将文字或前序完整结果确定性写为 Markdown 报告产物。"
	PipelineReportActionResult         = "artifactId 和 fileName 对象"
	PipelineScheduleMisfireMessage     = "skipped: 应用关闭期间错过触发"
)

// pipeline 模块的业务标识
const (
	PipelineScheduleKindOnce     = "once"
	PipelineScheduleKindInterval = "interval"
	PipelineMisfireSkip          = "skip"
	PipelineMisfireFireOnce      = "fire_once"
	PipelineChangedEvent         = "pipeline:changed"
)

// pipeline 模块的默认值与运行限制
const (
	PipelineSchedulePollInterval = time.Second
	PipelineActionCleanupTimeout = 20 * time.Second
	PipelineLocalTimeLayout      = "2006-01-02T15:04:05"
)

// pipeline 模块的状态与协议标识
const (
	PipelineActionSnapshot            = "workspace.snapshot"
	PipelineActionRunStep             = "step.run"
	PipelineActionReadKnowledge       = "knowledge.read"
	PipelineActionWriteReport         = "report.write"
	PipelineScheduleOutcomeTriggering = "triggering"
	PipelineScheduleOutcomeStarted    = "started"
	PipelineRequestKindSessionResult  = "pipeline_session_result"
	PipelineRequestKindRunResult      = "run_result"
	PipelineStateFilename             = "state.json"
	PipelineDirectoryName             = "pipelines"
)

const (
	PipelineStatusPaused  = "paused"
	PipelineControlPause  = "pause"
	PipelineControlResume = "resume"
	PipelineControlRetry  = "retry"
	PipelineControlStop   = "stop"
)

const (
	PipelineActionCreateWorktree      = "worktree.create"
	PipelineActionMergeWorktree       = "worktree.merge"
	PipelineActionRemoveWorktree      = "worktree.remove"
	PipelineCreateWorktreeDescription = "从项目当前分支创建本次运行专属的 Git 工作区与分支。"
	PipelineMergeWorktreeDescription  = "确认步骤成功、两侧工作区干净后，快进合并到来源分支；发生分歧时停止。"
	PipelineRemoveWorktreeDescription = "确认变更已合并后，删除本次运行的工作区目录和分支。"
	PipelineWorktreeResult            = "本次运行的来源分支、工作区目录、执行分支与合并提交。"
)
