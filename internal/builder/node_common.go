package builder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/process"
)

// detectPackageManager inspects project directory lockfiles to determine package manager.
func detectPackageManager(projectDir string) string {
	if _, err := os.Stat(filepath.Join(projectDir, "pnpm-lock.yaml")); err == nil {
		return "pnpm"
	}
	if _, err := os.Stat(filepath.Join(projectDir, "yarn.lock")); err == nil {
		return "yarn"
	}
	if _, err := os.Stat(filepath.Join(projectDir, "package-lock.json")); err == nil {
		return "npm"
	}
	return "npm"
}

// executeBuildSteps runs a series of planned steps for a project and collects execution metrics.
func executeBuildSteps(ctx context.Context, runner process.Runner, log logger.Logger, project models.Project, steps []Step, listener EventListener) (models.BuildResult, error) {
	if listener == nil {
		listener = &NopEventListener{}
	}

	result := models.BuildResult{
		Project:   project,
		Status:    models.BuildStatusRunning,
		Steps:     make([]models.BuildStepResult, 0, len(steps)),
		Artifacts: make([]models.Artifact, 0),
	}

	var cumulativeStdout, cumulativeStderr string
	totalSteps := len(steps)

	listener.OnBuildEvent(BuildEvent{
		Type:           EventProjectStarted,
		Time:           time.Now(),
		Project:        project.Name,
		ProjectDetails: project,
		Level:          "INFO",
		Message:        fmt.Sprintf("Building project %s", project.Name),
	})

	for i, step := range steps {
		stepIdx := i + 1
		cmdStr := step.ExecOptions.Command
		if len(step.ExecOptions.Args) > 0 {
			cmdStr += " " + strings.Join(step.ExecOptions.Args, " ")
		}

		listener.OnBuildEvent(BuildEvent{
			Type:           EventStepStarted,
			Time:           time.Now(),
			Project:        project.Name,
			ProjectDetails: project,
			Step:           step.Name,
			Command:        cmdStr,
			StepIndex:      stepIdx,
			TotalSteps:     totalSteps,
			Level:          "INFO",
			Message:        fmt.Sprintf("Executing step %s", step.Name),
		})

		stepOpts := step.ExecOptions
		if stepOpts.Dir == "" {
			stepOpts.Dir = project.RootDir
		}

		startTime := time.Now()
		stepRes, err := runner.Run(ctx, stepOpts)
		duration := time.Since(startTime)

		cumulativeStdout += stepRes.StdOut
		cumulativeStderr += stepRes.StdErr

		stepResult := models.BuildStepResult{
			Name:         step.Name,
			Command:      cmdStr,
			Duration:     duration,
			Successful:   stepRes.Successful,
			ExitCode:     stepRes.ExitCode,
			ErrorMessage: stepRes.ErrorMessage,
			StdOut:       stepRes.StdOut,
			StdErr:       stepRes.StdErr,
		}

		result.Steps = append(result.Steps, stepResult)
		result.Duration += duration

		if err != nil || !stepRes.Successful {
			result.Status = models.BuildStatusFailed
			errMsg := stepRes.ErrorMessage
			if errMsg == "" && err != nil {
				errMsg = err.Error()
			}
			result.ErrorMessage = fmt.Sprintf("step %q failed: %s", step.Name, errMsg)
			result.StdOut = cumulativeStdout
			result.StdErr = cumulativeStderr

			listener.OnBuildEvent(BuildEvent{
				Type:           EventStepFailed,
				Time:           time.Now(),
				Project:        project.Name,
				ProjectDetails: project,
				Step:           step.Name,
				Command:        cmdStr,
				StepIndex:      stepIdx,
				TotalSteps:     totalSteps,
				ExitCode:       stepRes.ExitCode,
				Duration:       duration,
				Error:          err,
				StepResult:     &stepResult,
				Level:          "ERROR",
				Message:        result.ErrorMessage,
			})

			return result, fmt.Errorf("build failed at step %q: %s", step.Name, errMsg)
		}

		listener.OnBuildEvent(BuildEvent{
			Type:           EventStepCompleted,
			Time:           time.Now(),
			Project:        project.Name,
			ProjectDetails: project,
			Step:           step.Name,
			Command:        cmdStr,
			StepIndex:      stepIdx,
			TotalSteps:     totalSteps,
			ExitCode:       stepRes.ExitCode,
			Duration:       duration,
			StepResult:     &stepResult,
			Level:          "INFO",
			Message:        fmt.Sprintf("Step %s completed", step.Name),
		})
	}

	result.Status = models.BuildStatusSuccess
	result.StdOut = cumulativeStdout
	result.StdErr = cumulativeStderr

	listener.OnBuildEvent(BuildEvent{
		Type:           EventProjectCompleted,
		Time:           time.Now(),
		Project:        project.Name,
		ProjectDetails: project,
		Level:          "INFO",
		Message:        fmt.Sprintf("Completed building project %s", project.Name),
	})

	return result, nil
}
