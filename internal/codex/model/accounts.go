package model

type BalanceConfig struct {
	URL          string `json:"url"`          // 用户指定的余额接口
	Method       string `json:"method"`       // GET 或 POST
	HeaderName   string `json:"headerName"`   // 引用当前 API 密钥的请求头
	HeaderPrefix string `json:"headerPrefix"` // 密钥前缀，例如 Bearer 加空格
	Body         string `json:"body"`         // POST JSON，完整字符串值 $API_KEY 引用当前密钥
	BalancePath  string `json:"balancePath"`  // 余额的 JSON Pointer，留空表示未知
	UsedPath     string `json:"usedPath"`     // 已用量的 JSON Pointer
	TotalPath    string `json:"totalPath"`    // 总量的 JSON Pointer
	UnitPath     string `json:"unitPath"`     // 单位的 JSON Pointer
	Unit         string `json:"unit"`         // 接口未提供单位时使用的显示文字
}

type AccountProfile struct {
	ID            string        `json:"id"`            // 应用内稳定标识
	Name          string        `json:"name"`          // 显示名称
	Type          string        `json:"type"`          // chatgpt 或 api
	Provider      string        `json:"provider"`      // API 提供方显示名称
	BaseURL       string        `json:"baseURL"`       // Responses 服务基础地址
	Model         string        `json:"model"`         // 当前账号保存的模型偏好
	Balance       BalanceConfig `json:"balance"`       // 可选余额查询配置
	KeyConfigured bool          `json:"keyConfigured"` // 是否已保存 API 密钥
	Authenticated bool          `json:"authenticated"` // 是否已有本应用的 ChatGPT 登录凭据
}

type ProfileInput struct {
	ID       string        `json:"id"`       // 编辑时传入已有账号标识
	Name     string        `json:"name"`     // 显示名称
	Type     string        `json:"type"`     // 账号类型
	Provider string        `json:"provider"` // API 提供方名称
	BaseURL  string        `json:"baseURL"`  // API 基础地址
	Model    string        `json:"model"`    // 账号模型，可填写服务支持的自定义标识
	APIKey   string        `json:"apiKey"`   // 只作为输入，编辑时留空保留
	Balance  BalanceConfig `json:"balance"`  // 余额请求与字段映射
}

type LoginState struct {
	ProfileID string `json:"profileId"` // 当前登录的独立账号
	LoginID   string `json:"loginId"`   // 上游登录标识
	Status    string `json:"status"`    // pending、completed、error 或 cancelled
	URL       string `json:"url"`       // 原生登录流程返回的浏览器地址
	Error     string `json:"error"`     // 可读失败原因
}

type AccountsView struct {
	Profiles []AccountProfile `json:"profiles"` // 不包含密钥或登录令牌
	ActiveID string           `json:"activeId"` // 当前选择的独立账号
	ReuseCLI bool             `json:"reuseCLI"` // 是否沿用 CLI 配置
	Login    LoginState       `json:"login"`    // 当前交互式登录进度
}

type BalanceResult struct {
	ProfileID string   `json:"profileId"` // 余额所属账号
	Balance   *float64 `json:"balance"`   // 接口返回的余额，未知时为空
	Used      *float64 `json:"used"`      // 接口返回的已用量
	Total     *float64 `json:"total"`     // 接口返回的总量
	Unit      string   `json:"unit"`      // 接口或用户指定的单位
	CheckedAt string   `json:"checkedAt"` // 实际查询时间
	Error     string   `json:"error"`     // 查询失败原因，与聊天状态独立
}

type RateWindow struct {
	UsedPercent        float64 `json:"usedPercent"`        // 上游已使用百分比
	WindowDurationMins *int64  `json:"windowDurationMins"` // 上游窗口时长
	ResetsAt           *int64  `json:"resetsAt"`           // Unix 秒重置时间
}

type CreditsSnapshot struct {
	HasCredits bool    `json:"hasCredits"` // 上游是否报告有额度
	Unlimited  bool    `json:"unlimited"`  // 是否无限额度
	Balance    *string `json:"balance"`    // 上游余额文字，不推算单位
}

type RateSnapshot struct {
	LimitID   *string          `json:"limitId"`   // 计量桶标识
	LimitName *string          `json:"limitName"` // 上游显示名称
	Primary   *RateWindow      `json:"primary"`   // 主窗口
	Secondary *RateWindow      `json:"secondary"` // 次窗口
	Credits   *CreditsSnapshot `json:"credits"`   // 上游附加额度
}

type RateLimits struct {
	RateLimits          *RateSnapshot           `json:"rateLimits"`          // 单桶响应
	RateLimitsByLimitID map[string]RateSnapshot `json:"rateLimitsByLimitId"` // 多桶响应优先展示
	CheckedAt           string                  `json:"checkedAt"`           // 实际查询时间
}
