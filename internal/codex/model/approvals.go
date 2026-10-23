package model

type QuestionOption struct {
	Label       string `json:"label"`       // 选项显示文字
	Description string `json:"description"` // 说明文字
}

type Question struct {
	ID       string           `json:"id"`       // 问题标识
	Header   string           `json:"header"`   // 问题分组标题
	Question string           `json:"question"` // 问题正文
	IsOther  bool             `json:"isOther"`  // 是否允许填写其他答案
	IsSecret bool             `json:"isSecret"` // 是否隐藏答案文字
	Options  []QuestionOption `json:"options"`  // 可用选项
}

type Approval struct {
	ID                 string             `json:"id"`                 // 审批请求标识
	Method             string             `json:"method"`             // 协议请求方法
	ThreadID           string             `json:"threadId"`           // 所属会话标识
	TurnID             string             `json:"turnId"`             // 所属轮次标识
	ItemID             string             `json:"itemId"`             // 所属过程节点标识
	Command            string             `json:"command"`            // 待执行命令
	Reason             string             `json:"reason"`             // 申请原因
	Cwd                string             `json:"cwd"`                // 执行目录
	GrantRoot          string             `json:"grantRoot"`          // 申请授权的根目录
	AvailableDecisions []string           `json:"availableDecisions"` // 上游允许的决策
	Questions          []Question         `json:"questions"`          // 待回答的问题
	Status             string             `json:"status"`             // 当前处理状态
	Generation         uint64             `json:"generation"`         // 所属连接代次
	DecisionOptions    []DecisionOption   `json:"decisionOptions"`    // 可展示的审批选项
	PermissionsJSON    string             `json:"permissionsJSON"`    // 原始权限请求
	Details            string             `json:"details"`            // 补充请求详情
	ServerName         string             `json:"serverName"`         // 发起请求的 MCP 服务
	Mode               string             `json:"mode"`               // MCP 问答模式
	Message            string             `json:"message"`            // 可读提示信息
	URL                string             `json:"url"`                // 请求提供的链接
	SchemaJSON         string             `json:"schemaJSON"`         // 原始表单结构
	Fields             []ElicitationField `json:"fields"`             // 可展示的表单字段
	Changes            []FileChange       `json:"changes"`            // 待审批的文件变更
}

type DecisionOption struct {
	Value   string `json:"value"`   // 选项值
	Label   string `json:"label"`   // 选项显示文字
	Details string `json:"details"` // 补充请求详情
}

type QuestionAnswer struct {
	Answers []string `json:"answers"` // 用户选择或填写的答案
}
