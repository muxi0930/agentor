package model

type StepCompletion struct {
	Completed    bool   // 是否已完成
	CompletedAt  string // 完成时间，未完成时为空
	DispatchedAt string // 最后一次派发时间
}
