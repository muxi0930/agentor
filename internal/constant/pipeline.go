package constant

import (
	"time"
)

// pipeline 模块的固定配置与提示文案
const (
	PipelineRunInterruptedMessage      = "应用退出时流水线尚未结束，未自动重放"
	PipelineRunShutdownMessage         = "应用退出，流水线已取消"
	PipelineReportHeaderTemplate       = "流水线 **%s**：%s\n\n"
	PipelineReportErrorTemplate        = "\n错误：%s\n"
	PipelineReportArtifactsTemplate    = "\n报告产物：%s\n"
	PipelineInspectActionDescription   = "读取当前项目的结构及步骤最新状态。"
	PipelineInspectActionResult        = "项目结构和步骤状态对象"
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
	PipelineLocalTimeLayout      = "2006-01-02T15:04:05"
)

// pipeline 模块的状态与协议标识
const (
	PipelineActionSnapshot            = "workspace.snapshot"
	PipelineActionReadKnowledge       = "knowledge.read"
	PipelineActionWriteReport         = "report.write"
	PipelineScheduleOutcomeTriggering = "triggering"
	PipelineScheduleOutcomeStarted    = "started"
	PipelineStateFilename             = "graph-state.json"
	PipelineDirectoryName             = "pipelines"
)

const (
	PipelineStatusPaused  = "paused"
	PipelineControlPause  = "pause"
	PipelineControlResume = "resume"
	PipelineControlStop   = "stop"
)

const (
	PipelineActionCreateWorktree      = "worktree.create"
	PipelineActionMergeWorktree       = "worktree.merge"
	PipelineActionRemoveWorktree      = "worktree.remove"
	PipelineCreateWorktreeDescription = "从执行目录当前分支创建本次运行专属的 Git 工作区与分支。"
	PipelineMergeWorktreeDescription  = "两侧工作区干净时快进合并；dirty 和 diverged 是可连线处理的固定结果。"
	PipelineRemoveWorktreeDescription = "确认变更已合并后，删除本次运行的工作区目录和分支。"
	PipelineWorktreeResult            = "本次运行的来源分支、工作区目录、执行分支与合并提交。"
)

const (
	PipelineKindStatic                = "static"
	PipelineKindAgent                 = "agent"
	PipelineKindEnd                   = "end"
	PipelineActionAgent               = "agent.run"
	PipelineActionResolveConflicts    = "agent.resolve-conflicts"
	PipelineEndSuccess                = "end.success"
	PipelineEndFailure                = "end.failure"
	PipelineBranchSuccess             = "success"
	PipelineBranchFailure             = "failure"
	PipelineBranchDirty               = "dirty"
	PipelineBranchDiverged            = "diverged"
	PipelineAgentMaxRetries           = 10
	PipelineAgentInstructions         = "你是流水线当前节点的执行 Agent。仅执行本节点的提示词与验收标准，输入及绑定步骤上下文由宿主提供。完成工作后必须调用 pipeline_complete_node：验收通过 outcome=success，branch 必须选自给定业务分支；未通过 outcome=failure，说明原因。output 可为自由文本或 JSON。工具确认只代表收到汇报，正常结束本轮后 Go 才会推进。不得自行运行后续节点、修改流水线定义或运行记录。只允许一次最终汇报。"
	PipelineAgentPrompt               = "节点：%s\n\n执行要求：\n%s\n\n验收标准：\n%s\n\n已解析输入：\n%s\n\n可选业务分支：\n%s\n\n绑定步骤上下文：\n%s\n\n上一次失败：\n%s"
	PipelineBranchSuccessDescription  = "执行完成"
	PipelineBranchDirtyDescription    = "工作区有未提交变更"
	PipelineBranchDivergedDescription = "来源分支已分歧，不能快进合并"
)

const (
	PipelineSystemPrepare         = "prepare"
	PipelineSystemMerge           = "merge"
	PipelineSystemPrefix          = "system.version."
	PipelineSystemPrepareName     = "准备版本工作区"
	PipelineSystemMergeName       = "检查并合并版本"
	PipelineSystemResolvePrompt   = "仅在给定版本 worktree 内，将记录的 sourceBranch 合并进当前工作分支，解决冲突并提交整合结果。不得修改来源工作区或清理 worktree，不得回滚已有业务改动。按仓库说明执行已有测试与构建；如无法确认验证通过，汇报 failure 并说明。完成后工作区必须干净，来源分支提交必须已包含在当前工作分支中。由宿主执行最终快进合并。"
	PipelineSystemResolveCriteria = "冲突全部解决，已有相关测试与构建通过，整合提交已完成，工作区干净；汇报实际验证命令和结果"
)

const (
	PipelineSystemDoneName         = "完成"
	PipelineSystemCreateName       = "创建或复用版本工作区"
	PipelineSystemCheckMergeName   = "检查完成条件与合并"
	PipelineSystemResolveName      = "整合分支并解决冲突"
	PipelineSystemCleanupName      = "清理已合并工作区"
	PipelineSystemResolveTimeout   = 1800
	PipelineVersionBranchPrefix    = "codex/version-"
	PipelineVersionDirectoryPrefix = "version-"
)
