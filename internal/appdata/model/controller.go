package model

type ControllerSettings struct {
	SecurityPrompt *string  `json:"securityPrompt,omitempty"` // 用户保存的安全提示词
	Models         []string `json:"models"`                   // 执行者可选模型，空字符串表示跟随默认会话
	Efforts        []string `json:"efforts"`                  // 执行者可选推理强度，空字符串表示跟随默认会话
	Prompt         string   `json:"prompt"`                   // 控制中心默认提示词
	DecisionPrompt string   `json:"decisionPrompt"`           // 模型与推理强度决策的自定义提示词
}
