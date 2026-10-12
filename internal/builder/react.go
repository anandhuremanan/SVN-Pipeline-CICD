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

// ReactBuilder handles building React (Vite) applications.
type ReactBuilder struct {
	runner process.Runner
	logger logger.Logger
}

// NewReactBuilder creates a ReactBuilder instance with injected dependencies.
func NewReactBuilder(runner process.Runner, log logger.Logger) *ReactBuilder {
	if log == nil {
		log = logger.NopLogger()
	}
	return &ReactBuilder{
		runner: runner,
		logger: log,
	}
}

func (b *ReactBuilder) Name() string {
	return "React Builder"
}

func (b *ReactBuilder) CanBuild(project models.Project) bool {
	return project.Type == models.ProjectTypeReact
}

func (b *ReactBuilder) Plan(project models.Project) ([]Step, error) {
	pkgMgr := detectPackageManager(project.RootDir)
	buildScript := scriptvalidator.ResolveScript(project)

	steps := []Step{
		installStep(pkgMgr, project.RootDir),
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

func (b *ReactBuilder) Build(ctx context.Context, project models.Project, listener EventListener) (models.BuildResult, error) {
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

	for _, candidate := range []string{"dist", "build"} {
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
