package builder

import (
	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// Step models an individual executable action within a project build workflow.
type Step struct {
	// Name is the human-readable display name of the step (e.g. "Install Dependencies").
	Name string `json:"name"`
	// ExecOptions contains the command execution parameters for running this step.
	ExecOptions models.ExecOptions `json:"exec_options"`
}
