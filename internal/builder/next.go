package builder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/process"
	"github.com/anandhuremanan/cicd-engine/internal/scriptvalidator"
)

// NextBuilder handles building Next.js web applications.
type NextBuilder struct {
	runner process.Runner
	logger logger.Logger
}

// NewNextBuilder creates a NextBuilder instance with injected dependencies.
func NewNextBuilder(runner process.Runner, log logger.Logger) *NextBuilder {
	if log == nil {
		log = logger.NopLogger()
	}
	return &NextBuilder{
		runner: runner,
		logger: log,
	}
}

func (b *NextBuilder) Name() string {
	return "Next.js Builder"
}

func (b *NextBuilder) CanBuild(project models.Project) bool {
	return project.Type == models.ProjectTypeNext
}

func (b *NextBuilder) Plan(project models.Project) ([]Step, error) {
	pkgMgr := detectPackageManager(project.RootDir)
	buildScript := scriptvalidator.ResolveScript(project)

	steps := []Step{
		{
			Name: "Install Dependencies",
			ExecOptions: models.ExecOptions{
				Command: pkgMgr,
				Args:    []string{"install"},
				Dir:     project.RootDir,
			},
		},
		{
			Name: "Build",
			ExecOptions: models.ExecOptions{
				Command: pkgMgr,
				Args:    []string{"run", buildScript},
				Dir:     project.RootDir,
			},
		},
	}
	return steps, nil
}

func (b *NextBuilder) Build(ctx context.Context, project models.Project, listener EventListener) (models.BuildResult, error) {
	steps, err := b.Plan(project)
	if err != nil {
		return models.BuildResult{}, err
	}

	// Emit build script info before steps begin
	buildScript := scriptvalidator.ResolveScript(project)
	scriptLabel := fmt.Sprintf("%s (default)", buildScript)
	if scriptvalidator.IsOverridden(project) {
		scriptLabel = fmt.Sprintf("%s (from cicd.yaml)", buildScript)
	}
	if listener != nil {
		listener.OnBuildEvent(BuildEvent{
			Type:           EventInfo,
			Time:           time.Now(),
			Project:        project.Name,
			ProjectDetails: project,
			Level:          "INFO",
			Message:        fmt.Sprintf("Using build script: %s", scriptLabel),
		})
	}

	result, err := executeBuildSteps(ctx, b.runner, b.logger, project, steps, listener)

	for _, candidate := range []string{".next", "out"} {
		path := filepath.Join(project.RootDir, candidate)
		if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
			result.Artifacts = append(result.Artifacts, models.Artifact{
				Name: candidate,
				Path: path,
			})
		}
	}

	return result, err
}
