package n8n

import "time"

// Execution is an item of GET /executions without includeData.
type Execution struct {
	ID         string     `json:"id"`
	Finished   bool       `json:"finished"`
	Mode       string     `json:"mode"`
	RetryOf    *string    `json:"retryOf"`
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"startedAt"`
	StoppedAt  *time.Time `json:"stoppedAt"`
	WorkflowID string     `json:"workflowId"`
	WaitTill   *time.Time `json:"waitTill"`
}

// Duration returns stoppedAt - startedAt, or false while the execution has not stopped.
func (e Execution) Duration() (time.Duration, bool) {
	if e.StartedAt == nil || e.StoppedAt == nil {
		return 0, false
	}
	return e.StoppedAt.Sub(*e.StartedAt), true
}

// Workflow is an item of GET /workflows. Nodes and connections are not decoded.
type Workflow struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Active       bool             `json:"active"`
	IsArchived   bool             `json:"isArchived"`
	CreatedAt    time.Time        `json:"createdAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
	TriggerCount int              `json:"triggerCount"`
	Tags         []Tag            `json:"tags"`
	Settings     WorkflowSettings `json:"settings"`
	Shared       []struct {
		ProjectID string `json:"projectId"`
		Role      string `json:"role"`
	} `json:"shared"`
}

// ProjectID returns the owning project, if n8n reported one.
func (w Workflow) ProjectID() string {
	for _, s := range w.Shared {
		if s.Role == "workflow:owner" {
			return s.ProjectID
		}
	}
	if len(w.Shared) > 0 {
		return w.Shared[0].ProjectID
	}
	return ""
}

// WorkflowSettings holds the settings relevant for execution statistics.
type WorkflowSettings struct {
	SaveDataSuccessExecution string `json:"saveDataSuccessExecution"`
	SaveDataErrorExecution   string `json:"saveDataErrorExecution"`
}

type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// InsightsValue is one metric of GET /insights/summary.
type InsightsValue struct {
	Value     float64  `json:"value"`
	Deviation *float64 `json:"deviation"`
	Unit      string   `json:"unit"`
}

type InsightsSummary struct {
	Total          InsightsValue `json:"total"`
	Failed         InsightsValue `json:"failed"`
	FailureRate    InsightsValue `json:"failureRate"`
	TimeSaved      InsightsValue `json:"timeSaved"`
	AverageRunTime InsightsValue `json:"averageRunTime"`
}

// AuditReport is one report of POST /audit, keyed by report name in the response.
type AuditReport struct {
	Risk     string         `json:"risk"`
	Sections []AuditSection `json:"sections"`
}

type AuditSection struct {
	Title          string           `json:"title"`
	Description    string           `json:"description"`
	Recommendation string           `json:"recommendation"`
	Location       []map[string]any `json:"location"`
}

type page[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"nextCursor"`
}
