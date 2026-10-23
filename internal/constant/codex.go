package constant

import (
	"time"
)

// codex 模块的固定配置与提示文案
const (
	CodexThreadSubscriptionLimit           = 50
	CodexUnsupportedServerRequestPrefix    = "agentor 尚不支持该服务器请求: "
	CodexAllowOnceLabel                    = "允许本次"
	CodexAllowSessionLabel                 = "本会话允许"
	CodexDeclineLabel                      = "拒绝"
	CodexCancelLabel                       = "拒绝并停止"
	CodexSaveCommandRuleLabel              = "允许并保存命令规则"
	CodexSaveNetworkRuleLabel              = "保存网络规则"
	CodexAdditionalPermissionsHeading      = "额外权限：\n"
	CodexNetworkRequestHeading             = "\n网络请求：\n"
	CodexDisconnectedMessage               = "尚未连接 Codex"
	CodexClosingMessage                    = "应用正在退出"
	CodexInitializationDisconnectedMessage = "Codex 在初始化完成前已断开"
	CodexLoginRebuiltMessage               = "连接已重建，请重新发起登录"
	CodexConnectingMessage                 = "正在启动 Codex app-server"
	CodexLoginDisconnectedMessage          = "原生连接已断开，请重新发起登录"
	CodexServerDisconnectedPrefix          = "Codex app-server 已断开: "
	CodexConnectedMessage                  = "Codex app-server 已连接"
	CodexThreadSummaryPrefix               = "会话 "
	CodexCustomModelSuffix                 = "（自定义）"
	CodexCustomModelDescription            = "由当前 API 账号配置，实际可用性以请求结果为准"
	CodexAttachmentInstructions            = "用户选定的本地附件，可按需读取："
	CodexAttachmentFallbackPrompt          = "请查看所附文件。"
	CodexBalanceFailurePrefix              = "余额查询失败："
	CodexRedactedKeyLabel                  = "[密钥]"
	CodexChatGPTConnectionSuccess          = "已从当前 ChatGPT 账号读取真实额度"
	CodexAPIConnectionSuccess              = "模型接口请求成功；实际模型调用仍以对话结果为准"
	CodexLoginEndedMessage                 = "原登录流程已经结束，请重新发起登录"
	CodexLoginFailedMessage                = "登录未完成，请重新尝试"
)

// codex 模块的业务标识
const (
	CodexPhaseReady           = "ready"
	CodexPhaseStarting        = "starting"
	CodexPhaseStopped         = "stopped"
	CodexStatusPending        = "pending"
	CodexStatusCompleted      = "completed"
	CodexStatusCancelled      = "cancelled"
	CodexStatusError          = "error"
	CodexEventStatus          = "codex:status"
	CodexEventMessage         = "codex:event"
	CodexEventLoginUpdated    = "host/loginUpdated"
	CodexItemUserMessage      = "userMessage"
	CodexItemAgentMessage     = "agentMessage"
	CodexItemReasoning        = "reasoning"
	CodexItemCommandExecution = "commandExecution"
	CodexItemFileChange       = "fileChange"
	CodexItemInputText        = "inputText"
)

// codex 模块的默认值与运行限制
const (
	CodexBinaryCheckTimeout   = 30 * time.Second
	CodexShellPathTimeout     = 5 * time.Second
	CodexConnectTimeout       = 30 * time.Second
	CodexRequestTimeout       = 60 * time.Second
	CodexShutdownTimeout      = 2 * time.Second
	CodexBalanceCacheDuration = time.Minute
	CodexHTTPTimeout          = 15 * time.Second
)

// codex 模块的状态与协议标识
const (
	CodexAccountTypeAPI                            = "api"
	CodexAccountTypeChatGPT                        = "chatgpt"
	CodexAccountsFilename                          = "accounts.json"
	CodexAuthFilename                              = "auth.json"
	CodexConfigFilename                            = "config.toml"
	CodexApprovalExpired                           = "expired"
	CodexApprovalResolved                          = "resolved"
	CodexServiceTierFast                           = "fast"
	CodexServiceTierDefault                        = "default"
	CodexApprovalAccept                            = "accept"
	CodexApprovalDecline                           = "decline"
	CodexApprovalCancel                            = "cancel"
	CodexApprovalAcceptForSession                  = "acceptForSession"
	CodexMethodAccountLoginCancel                  = "account/login/cancel"
	CodexMethodAccountLoginCompleted               = "account/login/completed"
	CodexMethodAccountLoginStart                   = "account/login/start"
	CodexMethodAccountRateLimitsRead               = "account/rateLimits/read"
	CodexMethodAccountRead                         = "account/read"
	CodexMethodItemCommandExecutionRequestApproval = "item/commandExecution/requestApproval"
	CodexMethodItemCompleted                       = "item/completed"
	CodexMethodItemFileChangeRequestApproval       = "item/fileChange/requestApproval"
	CodexMethodItemPermissionsRequestApproval      = "item/permissions/requestApproval"
	CodexMethodItemStarted                         = "item/started"
	CodexMethodItemToolCall                        = "item/tool/call"
	CodexMethodItemToolRequestUserInput            = "item/tool/requestUserInput"
	CodexMethodMcpServerElicitationRequest         = "mcpServer/elicitation/request"
	CodexMethodModelList                           = "model/list"
	CodexMethodThreadArchive                       = "thread/archive"
	CodexMethodThreadClosed                        = "thread/closed"
	CodexMethodThreadList                          = "thread/list"
	CodexMethodThreadNameSet                       = "thread/name/set"
	CodexMethodThreadNameUpdated                   = "thread/name/updated"
	CodexMethodThreadRead                          = "thread/read"
	CodexMethodThreadResume                        = "thread/resume"
	CodexMethodThreadStart                         = "thread/start"
	CodexMethodThreadTokenUsageUpdated             = "thread/tokenUsage/updated"
	CodexMethodThreadTurnsList                     = "thread/turns/list"
	CodexMethodThreadUnsubscribe                   = "thread/unsubscribe"
	CodexMethodTurnCompleted                       = "turn/completed"
	CodexMethodTurnInterrupt                       = "turn/interrupt"
	CodexMethodTurnStart                           = "turn/start"
	CodexMethodTurnStarted                         = "turn/started"
	CodexMethodTurnSteer                           = "turn/steer"
)
