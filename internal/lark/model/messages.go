package model

import (
	tasksmodel "agentor/internal/tasks/model"
)

type Identity struct {
	ID   string `json:"id"`   // 稳定标识
	Name string `json:"name"` // 显示名称
}

type Settings struct {
	AppID            string     `json:"appId"`            // 当前飞书应用
	Region           string     `json:"region"`           // 服务区域
	AllowedUsers     []Identity `json:"allowedUsers"`     // 允许的单聊联系人
	AllowedChats     []Identity `json:"allowedChats"`     // 允许接收任务的群
	DefaultProjectID string     `json:"defaultProjectId"` // 默认项目范围
}

type SettingsInput struct {
	Settings           // 公开连接设置
	AppSecret   string `json:"appSecret"`   // 只在私有配置中保存的应用密钥
	ClearSecret bool   `json:"clearSecret"` // 明确清除已保存密钥
}

type Status struct {
	Phase      string `json:"phase"`      // 连接阶段
	Message    string `json:"message"`    // 可读连接说明
	BotOpenID  string `json:"botOpenId"`  // 真实机器人身份
	Generation uint64 `json:"generation"` // 当前连接代次
}

type Candidate struct {
	Kind     string `json:"kind"`     // 身份或资源类型
	ID       string `json:"id"`       // 稳定标识
	LastSeen string `json:"lastSeen"` // 最近接收时间
}

type Incoming struct {
	Source        tasksmodel.Source `json:"source"`        // 原始消息来源
	RequestID     string            `json:"requestId"`     // 预留的统一请求标识
	Status        string            `json:"status"`        // 处理状态
	Error         string            `json:"error"`         // 失败原因
	CreatedAt     string            `json:"createdAt"`     // 接收时间
	ReactionID    string            `json:"reactionId"`    // 原消息处理中表态
	ReactionState string            `json:"reactionState"` // 表态状态
}

type ApprovalRoute struct {
	Error          string            `json:"error"`          // 失败原因
	Key            string            `json:"key"`            // 包含运行时代次的审批路由标识
	ApprovalID     string            `json:"approvalId"`     // 原始上游审批标识
	Generation     uint64            `json:"generation"`     // 当前连接代次
	Source         tasksmodel.Source `json:"source"`         // 原始消息来源
	NotificationID string            `json:"notificationId"` // 对应通知
	MessageID      string            `json:"messageId"`      // 已发送的卡片消息
	Status         string            `json:"status"`         // 处理状态
}

type View struct {
	Settings         Settings          `json:"settings"`         // 公开连接设置
	SecretConfigured bool              `json:"secretConfigured"` // 是否已经保存密钥
	Status           Status            `json:"status"`           // 处理状态
	Binding          tasksmodel.Source `json:"binding"`          // 桌面任务的配对接收者
	Candidates       []Candidate       `json:"candidates"`       // 仅身份信息的候选列表
	Incoming         []Incoming        `json:"incoming"`         // 去重后的接收记录
	Approvals        []ApprovalRoute   `json:"approvals"`        // 真实审批的远程路由
}

type Pairing struct {
	Code      string `json:"code"`      // 仅在内存中有效的一次性配对码
	SenderID  string `json:"senderId"`  // 指定配对联系人
	ExpiresAt string `json:"expiresAt"` // 配对码失效时间
}

type Resource struct {
	Key  string // 包含运行时代次的审批路由标识
	Name string // 显示名称
	Kind string // 身份或资源类型
}

type Message struct {
	Source     tasksmodel.Source // 原始消息来源
	ChatType   string            // 单聊或群聊
	SenderType string            // 用户或机器人
	Text       string            // 按原顺序提取的正文
	Mentions   []string          // 消息明确提及的身份
	Resources  []Resource        // 消息中的图片和文件
}
