package reporter

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/builder"
	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// Reporter defines methods for displaying step progress, structured failures, and build summaries.
type Reporter interface {
	builder.EventListener
	ReportProjectStart(project string)
	ReportStepResult(project string, step models.BuildStepResult)
	ReportFailure(project, stepName, command string, exitCode int, duration time.Duration, errorOutput string)
	ReportSummary(job models.Job)
}

// ConsoleReporter formats live build progress, step timers, spinners, structured errors, and rich summaries.
type ConsoleReporter struct {
	out          io.Writer
	mu           sync.Mutex
	activeTicker *stepTicker
	currentProj  string
}

type stepTicker struct {
	stepName  string
	command   string
	startTime time.Time
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// NewConsoleReporter constructs a ConsoleReporter instance.
func NewConsoleReporter(out io.Writer) *ConsoleReporter {
	if out == nil {
		out = os.Stdout
	}
	return &ConsoleReporter{out: out}
}

func (r *ConsoleReporter) ReportProjectStart(project string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stopActiveTickerLocked()

	divider := strings.Repeat("━", 50)
	fmt.Fprintln(r.out, divider)
	fmt.Fprintf(r.out, "PROJECT: %s\n", project)
	fmt.Fprintln(r.out, divider)
	fmt.Fprintln(r.out, "")
	r.currentProj = project
}

func (r *ConsoleReporter) stopActiveTickerLocked() {
	if r.activeTicker != nil {
		close(r.activeTicker.stopChan)
		r.activeTicker.wg.Wait()
		r.activeTicker = nil
	}
}

func (r *ConsoleReporter) OnBuildEvent(event builder.BuildEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch event.Type {
	case builder.EventProjectStarted:
		r.stopActiveTickerLocked()
		divider := strings.Repeat("━", 50)
		fmt.Fprintln(r.out, divider)
		projType := string(event.ProjectDetails.Type)
		if projType != "" {
			fmt.Fprintf(r.out, "PROJECT: %s (%s)\n", event.Project, projType)
		} else {
			fmt.Fprintf(r.out, "PROJECT: %s\n", event.Project)
		}
		fmt.Fprintln(r.out, divider)
		fmt.Fprintln(r.out, "")
		r.currentProj = event.Project

	case builder.EventStepStarted:
		r.stopActiveTickerLocked()

		stepName := event.Step
		if stepName == "" {
			stepName = event.Message
		}

		if event.TotalSteps > 0 {
			fmt.Fprintf(r.out, "Step [%d/%d]: %s\n", event.StepIndex, event.TotalSteps, stepName)
		} else {
			fmt.Fprintf(r.out, "Step: %s\n", stepName)
		}

		if event.Command != "" {
			fmt.Fprintf(r.out, "Command:   %s\n", event.Command)
		}

		isTTY := isTerminalWriter(r.out)

		t := &stepTicker{
			stepName:  stepName,
			command:   event.Command,
			startTime: time.Now(),
			stopChan:  make(chan struct{}),
		}

		t.wg.Add(1)
		r.activeTicker = t

		go func(st *stepTicker, out io.Writer, tty bool) {
			defer st.wg.Done()
			ticker := time.NewTicker(250 * time.Millisecond)
			defer ticker.Stop()

			frameIdx := 0
			for {
				select {
				case <-st.stopChan:
					if tty {
						fmt.Fprint(out, "\r\033[K")
					}
					return
				case <-ticker.C:
					elapsed := time.Since(st.startTime)
					minutes := int(elapsed.Minutes())
					seconds := int(elapsed.Seconds()) % 60
					frame := spinnerFrames[frameIdx%len(spinnerFrames)]
					frameIdx++

					if tty {
						fmt.Fprintf(out, "\r%s %s... %02d:%02d", frame, st.stepName, minutes, seconds)
					}
				}
			}
		}(t, r.out, isTTY)

	case builder.EventStepOutput, builder.EventStepProgress:
		// A terminal already shows the spinner line; live output is for logs and the desktop app.
		if event.Message != "" && !isTerminalWriter(r.out) {
			fmt.Fprintf(r.out, "  %s\n", event.Message)
		}

	case builder.EventStepCompleted:
		r.stopActiveTickerLocked()
		stepName := event.Step
		if stepName == "" {
			stepName = event.Message
		}
		durStr := formatDurationHuman(event.Duration)
		fmt.Fprintf(r.out, "✓ %s (%s)\n\n", stepName, durStr)

	case builder.EventStepFailed:
		r.stopActiveTickerLocked()
		stepName := event.Step
		if stepName == "" {
			stepName = event.Message
		}
		durStr := formatDurationHuman(event.Duration)
		fmt.Fprintf(r.out, "✗ %s (%s)\n\n", stepName, durStr)

	case builder.EventInfo:
		r.stopActiveTickerLocked()
		if event.Message != "" {
			// Extract the label part after "Using build script: "
			const prefix = "Using build script: "
			if len(event.Message) > len(prefix) {
				label := event.Message[len(prefix):]
				fmt.Fprintf(r.out, "Using build script:\n%s\n\n", label)
			} else {
				fmt.Fprintln(r.out, event.Message)
			}
		}
	}
}

func (r *ConsoleReporter) ReportStepResult(project string, step models.BuildStepResult) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stopActiveTickerLocked()

	if step.Successful {
		durStr := formatDurationHuman(step.Duration)
		fmt.Fprintf(r.out, "✓ %s (%s)\n\n", step.Name, durStr)
	} else {
		r.ReportFailure(project, step.Name, step.Command, step.ExitCode, step.Duration, step.ErrorMessage)
	}
}

func (r *ConsoleReporter) ReportFailure(project, stepName, command string, exitCode int, duration time.Duration, errorOutput string) {
	divider := strings.Repeat("━", 50)
	fmt.Fprintln(r.out, divider)
	fmt.Fprintln(r.out, "BUILD FAILED")
	fmt.Fprintf(r.out, "Project:   %s\n", project)
	fmt.Fprintf(r.out, "Step:      %s\n", stepName)
	fmt.Fprintf(r.out, "Command:   %s\n", command)
	fmt.Fprintf(r.out, "Exit Code: %d\n", exitCode)
	if duration > 0 {
		fmt.Fprintf(r.out, "Duration:  %s\n", formatDurationHuman(duration))
	}
	fmt.Fprintln(r.out, divider)
	if errorOutput != "" {
		fmt.Fprintf(r.out, "%s\n", errorOutput)
		fmt.Fprintln(r.out, divider)
	}
	fmt.Fprintln(r.out, "")
}

func (r *ConsoleReporter) ReportSummary(job models.Job) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stopActiveTickerLocked()

	divider := strings.Repeat("━", 50)
	fmt.Fprintln(r.out, divider)
	fmt.Fprintln(r.out, "Build Summary")
	fmt.Fprintln(r.out, divider)

	fmt.Fprintf(r.out, "Job:        %s\n", job.ID)
	fmt.Fprintf(r.out, "Repository: %s\n", job.Repository)
	fmt.Fprintf(r.out, "Revision:   %s\n", job.Revision)
	if job.BuildNumber > 0 {
		fmt.Fprintf(r.out, "Build No:   %d\n", job.BuildNumber)
		fmt.Fprintf(r.out, "Build Dir:  %s\n", job.BuildDir)
	}

	succeeded := 0
	failed := 0
	for _, res := range job.BuildResults {
		if res.Status == models.BuildStatusSuccess {
			succeeded++
		} else {
			failed++
		}
	}

	fmt.Fprintf(r.out, "Projects:   %d\n", len(job.BuildResults))
	fmt.Fprintf(r.out, "Succeeded:  %d\n", succeeded)
	fmt.Fprintf(r.out, "Failed:     %d\n", failed)

	if job.WorkspaceRetained {
		fmt.Fprintf(r.out, "Workspace:  Retained (%s)\n", job.Workspace)
	} else if job.Workspace != "" {
		fmt.Fprintln(r.out, "Workspace:  Cleaned")
	}

	if len(job.DeploymentOutputs) > 0 {
		fmt.Fprintln(r.out, divider)
		fmt.Fprintln(r.out, "Deployment Outputs")
		for proj, pubPath := range job.DeploymentOutputs {
			fmt.Fprintf(r.out, "%s\n  %s\n", proj, pubPath)
		}
	}

	if len(job.StageTimings) > 0 {
		fmt.Fprintln(r.out, divider)
		fmt.Fprintln(r.out, "Stage Timings")
		for stage, dur := range job.StageTimings {
			fmt.Fprintf(r.out, "  %-25s %s\n", stage, formatDurationHuman(dur))
		}
	}

	fmt.Fprintln(r.out, divider)
	fmt.Fprintf(r.out, "Total Duration: %s\n", formatDurationHuman(job.Duration))
	fmt.Fprintf(r.out, "Artifacts:      %d\n", len(job.Artifacts))
	fmt.Fprintln(r.out, divider)
	fmt.Fprintln(r.out, "")
}

func formatDurationHuman(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func isTerminalWriter(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		stat, err := f.Stat()
		if err == nil {
			return (stat.Mode() & os.ModeCharDevice) != 0
		}
	}
	return false
}
