package builder

import (
	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// BuildPlan defines the batch execution parameters and target projects for a build run.
type BuildPlan struct {
	// Projects is the slice of detected projects scheduled for building.
	Projects []models.Project `json:"projects"`
	// Parallelism specifies max concurrency limit for parallel execution.
	Parallelism int `json:"parallelism"`
	// FailFast specifies whether execution stops immediately on first project failure.
	FailFast bool `json:"fail_fast"`
}

// NewBuildPlan constructs a BuildPlan with valid default values.
func NewBuildPlan(projects []models.Project, parallelism int, failFast bool) BuildPlan {
	if parallelism < 1 {
		parallelism = 1
	}
	return BuildPlan{
		Projects:    projects,
		Parallelism: parallelism,
		FailFast:    failFast,
	}
}
