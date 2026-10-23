package model

type Status struct {
	Phase      string `json:"phase"`      // 当前连接阶段
	Message    string `json:"message"`    // 可读提示信息
	Path       string `json:"path"`       // 本地资源路径
	Version    string `json:"version"`    // 原生程序版本
	Home       string `json:"home"`       // 原生运行时数据目录
	ReuseCLI   bool   `json:"reuseCLI"`   // 是否沿用 CLI 配置
	Generation uint64 `json:"generation"` // 所属连接代次
	Revision   uint64 `json:"revision"`   // 状态修订号
}

type Event struct {
	Generation uint64 `json:"generation"` // 所属连接代次
	Method     string `json:"method"`     // 协议请求方法
	Params     string `json:"params"`     // 原始请求参数
}

type Account struct {
	Type     string `json:"type"`     // 当前账号类型
	Email    string `json:"email"`    // 当前账号邮箱
	PlanType string `json:"planType"` // 当前账号套餐类型
}

type AccountInfo struct {
	Account            *Account `json:"account"`            // 当前认证账号
	RequiresOpenAIAuth bool     `json:"requiresOpenaiAuth"` // 是否需要 OpenAI 登录
}
