package builder

import (
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// EventType indicates the category of build progress update.
type EventType string

const (
	EventProjectStarted   EventType = "PROJECT_STARTED"
	EventStepStarted      EventType = "STEP_STARTED"
	EventStepCompleted    EventType = "STEP_COMPLETED"
	EventStepFailed       EventType = "STEP_FAILED"
	EventProjectCompleted EventType = "PROJECT_COMPLETED"
	// EventInfo is used for informational messages emitted before step execution.
	EventInfo EventType = "INFO"
	// EventStepOutput carries one line printed by the running step.
	EventStepOutput EventType = "STEP_OUTPUT"
	// EventStepProgress carries a short summary of how far the running step is, e.g. "Downloaded 312 packages".
	EventStepProgress EventType = "STEP_PROGRESS"
)

// BuildEvent represents a real-time progress update emitted during a project build step.
type BuildEvent struct {
	Type           EventType               `json:"type"`
	Time           time.Time               `json:"time"`
	Project        string                  `json:"project"`
	ProjectDetails models.Project          `json:"project_details"`
	Step           string                  `json:"step"`
	Command        string                  `json:"command"`
	StepIndex      int                     `json:"step_index"`
	TotalSteps     int                     `json:"total_steps"`
	ExitCode       int                     `json:"exit_code"`
	Duration       time.Duration           `json:"duration"`
	Error          error                   `json:"error,omitempty"`
	StepResult     *models.BuildStepResult `json:"step_result,omitempty"`
	Level          string                  `json:"level"`
	Message        string                  `json:"message"`
}

// EventListener handles real-time build events emitted by builders.
type EventListener interface {
	// OnBuildEvent receives a BuildEvent update.
	OnBuildEvent(event BuildEvent)
}

// NopEventListener discards all build events.
type NopEventListener struct{}

// OnBuildEvent discards the event.
func (n *NopEventListener) OnBuildEvent(event BuildEvent) {}
