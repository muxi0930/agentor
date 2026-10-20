package tasks

import "agentor/internal/workspace"

type Source struct {
	Type             string `json:"type"`
	AppID            string `json:"appId"`
	ChatID           string `json:"chatId"`
	SenderID         string `json:"senderId"`
	MessageID        string `json:"messageId"`
	ReplyToMessageID string `json:"replyToMessageId"`
}

type DispatchInput struct {
	StepID             string `json:"stepId"`
	Instruction        string `json:"instruction"`
	CompletionCriteria string `json:"completionCriteria"`
	Model              string `json:"model"`
	Effort             string `json:"effort"`
	Source             Source `json:"source"`
	RequestID          string `json:"requestId"`
	TimeoutSeconds     int    `json:"timeoutSeconds"`
}

type Execution struct {
	ID                 string                `json:"id"`
	StepID             string                `json:"stepId"`
	ProjectID          string                `json:"projectId"`
	FeatureID          string                `json:"featureId"`
	Instruction        string                `json:"instruction"`
	CompletionCriteria string                `json:"completionCriteria"`
	Model              string                `json:"model"`
	Effort             string                `json:"effort"`
	Permission         string                `json:"permission"`
	Source             Source                `json:"source"`
	RequestID          string                `json:"requestId"`
	Context            workspace.StepContext `json:"context"`
	Status             string                `json:"status"`
	ThreadID           string                `json:"threadId"`
	TurnID             string                `json:"turnId"`
	Result             string                `json:"result"`
	Error              string                `json:"error"`
	CreatedAt          string                `json:"createdAt"`
	StartedAt          string                `json:"startedAt"`
	CompletedAt        string                `json:"completedAt"`
	StopRequested      bool                  `json:"stopRequested"`
	TimeoutSeconds     int                   `json:"timeoutSeconds"`
}

type NotificationPart struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Channel    string `json:"channel"`
	Target     string `json:"target"`
	Status     string `json:"status"`
	Error      string `json:"error"`
	Content    string `json:"content"`
	ArtifactID string `json:"artifactId"`
}

type Notification struct {
	ID          string             `json:"id"`
	Title       string             `json:"title"`
	Body        string             `json:"body"`
	ExecutionID string             `json:"executionId"`
	RequestID   string             `json:"requestId"`
	CreatedAt   string             `json:"createdAt"`
	Read        bool               `json:"read"`
	Parts       []NotificationPart `json:"parts"`
}

type State struct {
	Executions         []Execution       `json:"executions"`
	Notifications      []Notification    `json:"notifications"`
	Paused             bool              `json:"paused"`
	Requests           []Request         `json:"requests"`
	ControllerThreadID string            `json:"controllerThreadId"`
	LastSteps          map[string]string `json:"lastSteps"`
}

type ControllerInput struct {
	Text      string `json:"text"`
	ProjectID string `json:"projectId"`
	StepID    string `json:"stepId"`
	Model     string `json:"model"`
	Effort    string `json:"effort"`
	Source    Source `json:"source"`
}

type Request struct {
	ID                    string `json:"id"`
	Kind                  string `json:"kind"`
	Text                  string `json:"text"`
	ProjectID             string `json:"projectId"`
	StepID                string `json:"stepId"`
	Model                 string `json:"model"`
	Effort                string `json:"effort"`
	Source                Source `json:"source"`
	Status                string `json:"status"`
	ThreadID              string `json:"threadId"`
	TurnID                string `json:"turnId"`
	Result                string `json:"result"`
	Error                 string `json:"error"`
	CreatedAt             string `json:"createdAt"`
	CompletedAt           string `json:"completedAt"`
	CompletionExecutionID string `json:"completionExecutionId"`
	NeedsTarget           bool   `json:"needsTarget"`
	LockedTarget          bool   `json:"lockedTarget"`
}

type Scope struct {
	Role        string
	ProjectID   string
	FeatureID   string
	StepID      string
	ExecutionID string
	RequestID   string
	Directory   string
}
