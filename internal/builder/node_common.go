package builder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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

// installStep plans the dependency install for a Node project. npm prints nothing while it
// downloads, so it is asked to log each package fetch; the lines are counted into progress events.
func installStep(pkgMgr, rootDir string) Step {
	step := Step{
		Name: "Install Dependencies",
		ExecOptions: models.ExecOptions{
			Command: pkgMgr,
			Args:    []string{"install"},
			Dir:     rootDir,
		},
	}
	if pkgMgr == "npm" {
		// Use packages already in the npm cache without asking the registry about each one, and
		// skip the security audit and funding notice, which a build has no use for.
		step.ExecOptions.Args = append(step.ExecOptions.Args, "--prefer-offline", "--no-audit", "--no-fund")
		step.ExecOptions.Env = []string{"npm_config_loglevel=http"}
	}
	return step
}

// isFetchLine reports whether line is npm's log of one package download.
func isFetchLine(line string) bool {
	return strings.HasPrefix(line, "npm http fetch ") || strings.HasPrefix(line, "npm http cache ")
}

func dropFetchLines(output string) string {
	if !strings.Contains(output, "npm http ") {
		return output
	}
	lines := strings.Split(output, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if !isFetchLine(strings.TrimSpace(line)) {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// stepStream turns the live output of a running step into build events: package downloads are
// counted into a progress summary at most once a second, every other line is passed on as is.
type stepStream struct {
	mu           sync.Mutex
	emit         func(eventType EventType, message string)
	packages     int
	reported     int
	lastProgress time.Time
}

func (s *stepStream) Stdout(line string) { s.line(line) }
func (s *stepStream) Stderr(line string) { s.line(line) }

// ansiCodes matches the colour and cursor sequences that build tools print for terminals.
var ansiCodes = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

func (s *stepStream) line(line string) {
	line = strings.TrimSpace(ansiCodes.ReplaceAllString(line, ""))
	if line == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !isFetchLine(line) {
		s.emit(EventStepOutput, line)
		return
	}
	s.packages++
	if time.Since(s.lastProgress) >= time.Second {
		s.reportLocked()
	}
}

func (s *stepStream) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.packages != s.reported {
		s.reportLocked()
	}
}

func (s *stepStream) reportLocked() {
	s.lastProgress = time.Now()
	s.reported = s.packages
	s.emit(EventStepProgress, fmt.Sprintf("Downloaded %d packages", s.packages))
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

		stream := &stepStream{
			emit: func(eventType EventType, message string) {
				listener.OnBuildEvent(BuildEvent{
					Type:           eventType,
					Time:           time.Now(),
					Project:        project.Name,
					ProjectDetails: project,
					Step:           step.Name,
					StepIndex:      stepIdx,
					TotalSteps:     totalSteps,
					Level:          "INFO",
					Message:        message,
				})
			},
			lastProgress: time.Now(),
		}

		startTime := time.Now()
		stepRes, err := runner.Run(ctx, stepOpts, stream)
		duration := time.Since(startTime)
		stream.finish()

		// The per-package fetch lines are progress noise, not part of the step's result.
		stepRes.StdOut = dropFetchLines(stepRes.StdOut)
		stepRes.StdErr = dropFetchLines(stepRes.StdErr)

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
