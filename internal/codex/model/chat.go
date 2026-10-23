package model

import (
	"encoding/json"

	"agentor/internal/constant"
)

type ReasoningOption struct {
	ReasoningEffort string `json:"reasoningEffort"` // 支持的思考强度
	Description     string `json:"description"`     // 说明文字
}

type ModelServiceTier struct {
	ID          string `json:"id"`          // 速度档位标识
	Name        string `json:"name"`        // 速度档位名称
	Description string `json:"description"` // 速度档位说明
}

type Model struct {
	ID                        string             `json:"id"`                        // 模型目录标识
	Model                     string             `json:"model"`                     // 模型标识
	DisplayName               string             `json:"displayName"`               // 模型显示名称
	Description               string             `json:"description"`               // 说明文字
	IsDefault                 bool               `json:"isDefault"`                 // 是否为默认模型
	Hidden                    bool               `json:"hidden"`                    // 是否在模型列表中隐藏
	DefaultReasoningEffort    string             `json:"defaultReasoningEffort"`    // 模型默认思考强度
	SupportedReasoningEfforts []ReasoningOption  `json:"supportedReasoningEfforts"` // 模型支持的思考强度
	InputModalities           []string           `json:"inputModalities"`           // 模型支持的输入类型
	ServiceTiers              []ModelServiceTier `json:"serviceTiers"`              // 模型支持的速度档位
}

type ThreadStatus struct {
	Type string `json:"type"` // 会话状态类型
}

type Thread struct {
	ID         string       `json:"id"`         // 会话标识
	Name       string       `json:"name"`       // 会话标题
	Preview    string       `json:"preview"`    // 会话预览文字
	Cwd        string       `json:"cwd"`        // 执行目录
	Model      string       `json:"model"`      // 模型标识
	Originator string       `json:"originator"` // 会话创建来源标识
	CreatedAt  int64        `json:"createdAt"`  // 创建时间
	UpdatedAt  int64        `json:"updatedAt"`  // 最后更新时间
	Status     ThreadStatus `json:"status"`     // 当前处理状态
}

type ThreadPage struct {
	Data       []Thread `json:"data"`       // 当前分页数据
	NextCursor *string  `json:"nextCursor"` // 下一页游标
}

type Input struct {
	Type string `json:"type"`           // 输入类型
	Text string `json:"text,omitempty"` // 文字内容
	Path string `json:"path,omitempty"` // 本地资源路径
}

type Item struct {
	ID               string       `json:"id"`                   // 消息或过程节点标识
	ClientID         string       `json:"clientId"`             // 客户端消息标识
	Type             string       `json:"type"`                 // 消息或过程节点类型
	Text             string       `json:"text"`                 // 文字内容
	Phase            string       `json:"phase"`                // 助手消息阶段
	Content          []Input      `json:"content"`              // 用户输入内容
	Command          string       `json:"command"`              // 待执行命令
	AggregatedOutput string       `json:"aggregatedOutput"`     // 累计命令输出
	Status           string       `json:"status"`               // 当前处理状态
	ExitCode         *int         `json:"exitCode"`             // 命令退出码
	Summary          []string     `json:"summary"`              // 思考摘要分段
	ReasoningContent []string     `json:"reasoningContent"`     // 上游提供的思考内容分段
	DurationMs       *int64       `json:"durationMs,omitempty"` // 执行耗时毫秒数
	Changes          []FileChange `json:"changes,omitempty"`    // 文件变更明细
	Path             string       `json:"path,omitempty"`       // 本地资源路径
	Query            string       `json:"query,omitempty"`      // 搜索关键词
	Server           string       `json:"server,omitempty"`     // MCP 服务名称
	Tool             string       `json:"tool,omitempty"`       // 宿主工具名称
	Details          string       `json:"details,omitempty"`    // 补充请求详情
}

type FileChangeKind struct {
	Type     string `json:"type"`                // 文件变更类型
	MovePath string `json:"move_path,omitempty"` // 移动后的文件路径
}

type FileChange struct {
	Path string         `json:"path"` // 本地资源路径
	Kind FileChangeKind `json:"kind"` // 文件变更类型
	Diff string         `json:"diff"` // 文件变更补丁
}

type TurnError struct {
	Message string `json:"message"` // 可读提示信息
}

type Turn struct {
	ID          string     `json:"id"`                    // 轮次标识
	Status      string     `json:"status"`                // 当前处理状态
	Items       []Item     `json:"items"`                 // 本轮消息与过程节点
	Error       *TurnError `json:"error"`                 // 失败原因
	StartedAt   *int64     `json:"startedAt,omitempty"`   // 开始时间
	CompletedAt *int64     `json:"completedAt,omitempty"` // 完成时间
	DurationMs  *int64     `json:"durationMs,omitempty"`  // 执行耗时毫秒数
}

type TurnPage struct {
	Data       []Turn  `json:"data"`       // 当前分页数据
	NextCursor *string `json:"nextCursor"` // 下一页游标
}

type Conversation struct {
	Thread       Thread      `json:"thread"`               // 原生会话元信息
	Turns        TurnPage    `json:"turns"`                // 会话轮次分页
	ActiveTurnID string      `json:"activeTurnId"`         // 当前活动轮次标识
	ReadOnly     bool        `json:"readOnly"`             // 是否因其他应用占用而只读
	TokenUsage   *TokenUsage `json:"tokenUsage,omitempty"` // 会话令牌用量
}

type TokenBreakdown struct {
	InputTokens           int64 `json:"inputTokens"`           // 输入令牌数
	OutputTokens          int64 `json:"outputTokens"`          // 输出令牌数
	CachedInputTokens     int64 `json:"cachedInputTokens"`     // 缓存输入令牌数
	CacheWriteInputTokens int64 `json:"cacheWriteInputTokens"` // 写入缓存的输入令牌数
	ReasoningOutputTokens int64 `json:"reasoningOutputTokens"` // 思考输出令牌数
	TotalTokens           int64 `json:"totalTokens"`           // 令牌总数
}

type TokenUsage struct {
	Total              TokenBreakdown `json:"total"`              // 累计令牌用量
	Last               TokenBreakdown `json:"last"`               // 最近一轮令牌用量
	ModelContextWindow *int64         `json:"modelContextWindow"` // 模型上下文窗口大小
}

// UnmarshalJSON 按当前协议读取文字与思考消息的不同字段
func (item *Item) UnmarshalJSON(data []byte) error {
	type plain Item
	payload := struct {
		*plain                  // 待解码的消息公共字段
		Content json.RawMessage `json:"content"` // MCP 问答提交内容
	}{plain: (*plain)(item)}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	switch item.Type {
	case constant.CodexItemUserMessage:
		if len(payload.Content) > 0 {
			return json.Unmarshal(payload.Content, &item.Content)
		}
	case constant.CodexItemReasoning:
		if len(payload.Content) > 0 {
			return json.Unmarshal(payload.Content, &item.ReasoningContent)
		}
	default:
		if item.Type != constant.CodexItemAgentMessage && item.Type != constant.CodexItemCommandExecution && item.Type != constant.CodexItemFileChange {
			item.Details = string(data)
		}
	}

	return nil
}
