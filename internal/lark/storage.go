package lark

import (
	larkmodel "agentor/internal/lark/model"
	tasksmodel "agentor/internal/tasks/model"
)

type credentials struct {
	larkmodel.Settings                   // 公开连接设置
	AppSecret          string            `json:"appSecret"` // 只在私有配置中保存的应用密钥
	Binding            tasksmodel.Source `json:"binding"`   // 桌面任务的配对接收者
}

type records struct {
	Candidates []larkmodel.Candidate     `json:"candidates"` // 仅身份信息的候选列表
	Incoming   []larkmodel.Incoming      `json:"incoming"`   // 去重后的接收记录
	Approvals  []larkmodel.ApprovalRoute `json:"approvals"`  // 真实审批的远程路由
}
