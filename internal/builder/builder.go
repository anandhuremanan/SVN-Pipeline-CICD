package builder

import (
	"context"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// Builder defines the interface for language and framework-specific build execution.
type Builder interface {
	// Name returns the descriptive name of the builder (e.g. "React Builder", "Dotnet Builder").
	Name() string
	// CanBuild returns true if this builder supports building the given project.
	CanBuild(project models.Project) bool
	// Plan generates an ordered list of executable Build steps for the project.
	Plan(project models.Project) ([]Step, error)
	// Build executes the project's build steps, emitting progress events to the listener.
	Build(ctx context.Context, project models.Project, listener EventListener) (models.BuildResult, error)
}
