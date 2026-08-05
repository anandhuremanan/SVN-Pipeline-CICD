package builder

import (
	"context"
	"os"
	"path/filepath"

	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/process"
)

// DotnetBuilder handles building and publishing .NET applications and solutions.
type DotnetBuilder struct {
	runner process.Runner
	logger logger.Logger
}

// NewDotnetBuilder creates a DotnetBuilder instance with injected dependencies.
func NewDotnetBuilder(runner process.Runner, log logger.Logger) *DotnetBuilder {
	if log == nil {
		log = logger.NopLogger()
	}
	return &DotnetBuilder{
		runner: runner,
		logger: log,
	}
}

func (b *DotnetBuilder) Name() string {
	return ".NET Builder"
}

func (b *DotnetBuilder) CanBuild(project models.Project) bool {
	return project.Type == models.ProjectTypeDotnet
}

func (b *DotnetBuilder) Plan(project models.Project) ([]Step, error) {
	pubDir := filepath.Join(project.RootDir, "publish")
	steps := []Step{
		{
			Name: "Restore",
			ExecOptions: models.ExecOptions{
				Command: "dotnet",
				Args:    []string{"restore"},
				Dir:     project.RootDir,
			},
		},
		{
			Name: "Publish",
			ExecOptions: models.ExecOptions{
				Command: "dotnet",
				Args:    []string{"publish", "-c", "Release", "-o", pubDir},
				Dir:     project.RootDir,
			},
		},
	}
	return steps, nil
}

func (b *DotnetBuilder) Build(ctx context.Context, project models.Project, listener EventListener) (models.BuildResult, error) {
	steps, err := b.Plan(project)
	if err != nil {
		return models.BuildResult{}, err
	}

	result, err := executeBuildSteps(ctx, b.runner, b.logger, project, steps, listener)

	pubPath := filepath.Join(project.RootDir, "publish")
	if info, statErr := os.Stat(pubPath); statErr == nil && info.IsDir() {
		result.Artifacts = append(result.Artifacts, models.Artifact{
			Name: "publish",
			Path: pubPath,
		})
	}

	return result, err
}
