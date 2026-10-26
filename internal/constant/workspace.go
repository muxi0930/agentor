package constant

// workspace 模块的固定配置与提示文案
const (
	WorkspaceImportMarkdownTitle = "导入 Markdown 知识文档"
)

// workspace 模块的业务标识
const (
	WorkspaceKindProject = "project"
	WorkspaceKindVersion = "version"
	WorkspaceKindFeature = "feature"
	WorkspaceKindStep    = "step"
	WorkspaceKindTag     = "tag"
)

// workspace 模块的状态与协议标识
const (
	WorkspaceStateFilename      = "workspace.json"
	WorkspaceKnowledgeDirectory = "knowledge"
)

const (
	WorkspaceWorktreePreparing = "preparing"
	WorkspaceWorktreeReady     = "ready"
	WorkspaceWorktreeMerged    = "merged"
	WorkspaceWorktreeCleaned   = "cleaned"
)
