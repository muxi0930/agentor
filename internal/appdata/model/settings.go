package model

type Settings struct {
	AgentTimeoutSeconds int                           `json:"agentTimeoutSeconds"` // 功能子代理最大执行秒数，零表示不限时
	MaxConcurrentAgents int                           `json:"maxConcurrentAgents"` // 工作队列功能任务的最大并发数
	Controller          ControllerSettings            `json:"controller"`          // 控制中心配置
	ActiveAccountID     string                        `json:"activeAccountId"`     // 当前独立账号，空值表示尚未选择
	AccountPreferences  map[string]AccountPreferences `json:"accountPreferences"`  // 每个连接身份的模型与权限偏好
	Theme               string                        `json:"theme"`               // 界面主题
	ColorTheme          string                        `json:"colorTheme"`          // 界面配色方案
	ChatWidth           string                        `json:"chatWidth"`           // 聊天消息区域宽度：standard 或 wide
	Mode                string                        `json:"mode"`                // 当前工作模式
	ProjectDirectory    string                        `json:"projectDirectory"`    // 当前项目目录
	CodexPath           string                        `json:"codexPath"`           // 原生 Codex 程序路径
	CodexHomeMode       string                        `json:"codexHomeMode"`       // Codex 数据目录模式：system 或 custom
	CodexHomePath       string                        `json:"codexHomePath"`       // 自定义 Codex 数据目录
	ReadExternalThreads bool                          `json:"readExternalThreads"` // 是否读取非本应用创建的会话
	ExternalThreads     map[string]bool               `json:"externalThreads"`     // 在本应用打开过的外部会话
	ReuseCLI            bool                          `json:"reuseCLI"`            // 是否沿用 CLI 配置
	Model               string                        `json:"model"`               // 当前模型
	Effort              string                        `json:"effort"`              // 当前推理强度
	FastMode            bool                          `json:"fastMode"`            // 是否启用快速模式
	LastThreadID        string                        `json:"lastThreadId"`        // 最近打开的会话
	ChatThreads         map[string]string             `json:"chatThreads"`         // 本应用登记的会话及其目录
	ProjectAliases      map[string]string             `json:"projectAliases"`      // 项目目录的显示别名
	HiddenProjects      []string                      `json:"hiddenProjects"`      // 已隐藏的项目目录
	PermissionMode      string                        `json:"permissionMode"`      // 当前账号的默认权限
	ProjectPermissions  map[string]string             `json:"projectPermissions"`  // 项目权限覆盖值
	ThreadPermissions   map[string]string             `json:"threadPermissions"`   // 会话权限覆盖值
	PinnedThreads       map[string]bool               `json:"pinnedThreads"`       // 已置顶的会话
	ChatOrder           map[string][]string           `json:"chatOrder"`           // 各项目内的会话顺序
	ProjectOrder        []string                      `json:"projectOrder"`        // 项目显示顺序
	ManagedThreads      map[string]string             `json:"managedThreads"`      // 业务专用会话及其角色
}

type Info struct {
	GlobalConfigPath     string `json:"globalConfigPath"`     // 全局启动配置文件
	DataDirectoryFromEnv bool   `json:"dataDirectoryFromEnv"` // 数据目录是否由环境变量指定
	Name                 string `json:"name"`                 // 应用名称
	Version              string `json:"version"`              // 应用版本
	GoVersion            string `json:"goVersion"`            // Go 运行时版本
	OS                   string `json:"os"`                   // 操作系统
	Arch                 string `json:"arch"`                 // 处理器架构
	DataDirectory        string `json:"dataDirectory"`        // 应用数据目录
	CodexHome            string `json:"codexHome"`            // 本应用的 Codex 数据目录
	SystemCodexHome      string `json:"systemCodexHome"`      // 系统默认 Codex 数据目录
}
