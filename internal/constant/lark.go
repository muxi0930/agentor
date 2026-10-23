package constant

import (
	"time"
)

// lark 模块的固定配置与提示文案
const (
	LarkApprovalTitle                   = "Codex 审批与问答"
	LarkApprovalIDLabel                 = "审批 ID："
	LarkThreadLabel                     = "会话："
	LarkTurnLabel                       = "轮次："
	LarkDirectoryLabel                  = "目录："
	LarkWriteScopeLabel                 = "写入范围："
	LarkDiffTruncatedMessage            = "\n…完整修改请在桌面审批中心查看"
	LarkFileLabel                       = "文件："
	LarkOperationLabel                  = "\n操作："
	LarkMoveTargetLabel                 = "移动至："
	LarkQuestionIDLabel                 = "\n问题 ID："
	LarkAnswerCommandPrefix             = "请在原会话发送 `/answer "
	LarkAnswerCommandSuffix             = " {\"问题ID\":[\"答案\"]}`，或在桌面问答中心提交。"
	LarkElicitationCommandPrefix        = "发送 `/elicitation "
	LarkElicitationCommandSuffix        = " JSON` 提交回答，或在桌面继续。"
	LarkEmptyApprovalInstructions       = "此授权无需填写内容，请点击「同意」「拒绝」或「取消」。"
	LarkRequestTruncatedMessage         = "\n\n…完整请求请在桌面审批中心查看"
	LarkAllowTurnLabel                  = "允许本轮"
	LarkDeclineLabel                    = "拒绝"
	LarkCancelLabel                     = "取消"
	LarkAcceptLabel                     = "同意"
	LarkApprovalActionInvalidMessage    = "无效的审批操作"
	LarkApprovalRepliedMessage          = "已回复审批，最终执行结果仍以任务记录为准"
	LarkApprovalStatusTitle             = "审批状态"
	LarkStatusLabel                     = "\n\n状态："
	LarkUnconfiguredMessage             = "请配置 App ID 和 App Secret"
	LarkReceiveInterruptedMessage       = "应用退出时接收尚未完成，未自动重复提交"
	LarkWaitingConnectionMessage        = "等待建立连接"
	LarkConnectionClosedMessage         = "连接已关闭"
	LarkConnectingMessage               = "正在读取机器人身份并建立长连接"
	LarkConnectedMessage                = "长连接已建立"
	LarkReconnectingMessage             = "SDK 正在重连"
	LarkReconnectedMessage              = "长连接已恢复"
	LarkDisconnectedMessage             = "长连接已断开"
	LarkDeliveryConfirmedMissingMessage = "用户确认未送达"
	LarkDeliverySendingMessage          = "发送中；未收到明确响应前请勿重发"
	LarkArtifactShareTitle              = "分享产物"
	LarkArtifactQueuedMessage           = "产物已进入发送队列"
	LarkConnectionTestMessage           = "agentor 连接验证：官方 Go SDK 已连接。请在此单聊回复一条测试任务；图片、文件和需要确认的操作会进入 agentor 的任务与审批流程。"
	LarkConnectionTestTitle             = "飞书连接验证"
	LarkImagePlaceholder                = "[图片 image.png]"
	LarkFilePlaceholderPrefix           = "[文件 "
	LarkImagePlaceholderPrefix          = "[图片 "
	LarkControlHandledMessage           = "操作已处理"
	LarkControlReplyTitle               = "飞书控制响应"
	LarkAttachmentFailurePrefix         = "\n附件 "
	LarkAttachmentFailureSeparator      = " 未能载入："
	LarkRequestRejectedTitle            = "飞书请求未提交"
	LarkUnknownModelLabel               = "未知模型"
	LarkUnknownEffortLabel              = "未知强度"
	LarkUnknownValueLabel               = "未知"
)

// lark 模块的业务标识
const (
	LarkStatusPending      = "pending"
	LarkStatusSent         = "sent"
	LarkStatusFailed       = "failed"
	LarkStatusIgnored      = "ignored"
	LarkStatusReceiving    = "receiving"
	LarkStatusInterrupted  = "interrupted"
	LarkStatusExpired      = "expired"
	LarkStatusUnconfigured = "unconfigured"
	LarkStatusConnecting   = "connecting"
	LarkStatusDisconnected = "disconnected"
	LarkStatusError        = "error"
	LarkChangedEvent       = "lark:changed"
)

// lark 模块的默认值与运行限制
const (
	LarkHTTPTimeout       = 20 * time.Second
	LarkDeliveryTimeout   = 30 * time.Second
	LarkPairingDuration   = 10 * time.Minute
	LarkPairingCodeLength = 8
)

// lark 模块的状态与协议标识
const (
	LarkStatusUnknown   = "unknown"
	LarkStatusSubmitted = "submitted"
	LarkReactionBusy    = "busy"
	LarkReactionDone    = "done"
	LarkConfigFilename  = "config.json"
	LarkRecordsFilename = "records.json"
	LarkRegionFeishu    = "feishu"
	LarkChatTypePrivate = "p2p"
	LarkChatTypeGroup   = "group"
)
