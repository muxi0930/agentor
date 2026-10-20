package pipeline

import "agentor/internal/tasks"

type InputField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}
type Action struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Params map[string]any `json:"params"`
}
type Task struct {
	ID        string       `json:"id"`
	ProjectID string       `json:"projectId"`
	Name      string       `json:"name"`
	Inputs    []InputField `json:"inputs"`
	Actions   []Action     `json:"actions"`
}
type Parameter struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}
type ActionSpec struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  []Parameter `json:"parameters"`
	Result      string      `json:"result"`
}
type RunInput struct {
	SessionRequestID string         `json:"sessionRequestId"`
	TaskID           string         `json:"taskId"`
	Inputs           map[string]any `json:"inputs"`
	Model            string         `json:"model"`
	Effort           string         `json:"effort"`
	Source           tasks.Source   `json:"source"`
	RequestID        string         `json:"requestId"`
}
type ActionResult struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Status      string         `json:"status"`
	Inputs      map[string]any `json:"inputs"`
	Output      any            `json:"output"`
	Error       string         `json:"error"`
	StartedAt   string         `json:"startedAt"`
	CompletedAt string         `json:"completedAt"`
}
type Run struct {
	SessionRequestID string         `json:"sessionRequestId"`
	ID               string         `json:"id"`
	ProjectID        string         `json:"projectId"`
	TaskID           string         `json:"taskId"`
	Task             Task           `json:"task"`
	Inputs           map[string]any `json:"inputs"`
	Model            string         `json:"model"`
	Effort           string         `json:"effort"`
	Source           tasks.Source   `json:"source"`
	RequestID        string         `json:"requestId"`
	ScheduleID       string         `json:"scheduleId"`
	Status           string         `json:"status"`
	Error            string         `json:"error"`
	CreatedAt        string         `json:"createdAt"`
	CompletedAt      string         `json:"completedAt"`
	Actions          []ActionResult `json:"actions"`
	ArtifactIDs      []string       `json:"artifactIds"`
}
type Schedule struct {
	ID              string         `json:"id"`
	ProjectID       string         `json:"projectId"`
	TaskID          string         `json:"taskId"`
	Name            string         `json:"name"`
	Kind            string         `json:"kind"`
	Enabled         bool           `json:"enabled"`
	At              string         `json:"at"`
	Anchor          string         `json:"anchor"`
	IntervalSeconds int64          `json:"intervalSeconds"`
	Timezone        string         `json:"timezone"`
	MisfirePolicy   string         `json:"misfirePolicy"`
	Inputs          map[string]any `json:"inputs"`
	NextAt          string         `json:"nextAt"`
	LastAt          string         `json:"lastAt"`
	LastOutcome     string         `json:"lastOutcome"`
	LastRunID       string         `json:"lastRunId"`
}
type SessionInput struct {
	ProjectID string       `json:"projectId"`
	Role      string       `json:"role"`
	Text      string       `json:"text"`
	Model     string       `json:"model"`
	Effort    string       `json:"effort"`
	Source    tasks.Source `json:"source"`
	RequestID string       `json:"requestId"`
}
type SessionRequest struct {
	Kind            string       `json:"kind"`
	CompletionRunID string       `json:"completionRunId"`
	ID              string       `json:"id"`
	Input           SessionInput `json:"input"`
	Status          string       `json:"status"`
	ThreadID        string       `json:"threadId"`
	TurnID          string       `json:"turnId"`
	Result          string       `json:"result"`
	Error           string       `json:"error"`
	CreatedAt       string       `json:"createdAt"`
	CompletedAt     string       `json:"completedAt"`
}
type State struct {
	Tasks     []Task            `json:"tasks"`
	Schedules []Schedule        `json:"schedules"`
	Runs      []Run             `json:"runs"`
	Requests  []SessionRequest  `json:"requests"`
	Threads   map[string]string `json:"threads"`
	Paused    bool              `json:"paused"`
}
