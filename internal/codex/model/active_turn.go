package model

type ActiveTurn struct {
	ThreadID string `json:"threadId"` // 活动会话标识
	Summary  string `json:"summary"`  // 当前用户输入的简述
}
