package reporter_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/builder"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/reporter"
)

func TestConsoleReporterFormatting(t *testing.T) {
	var buf bytes.Buffer
	rep := reporter.NewConsoleReporter(&buf)

	rep.ReportProjectStart("CLIENT")
	rep.OnBuildEvent(builder.BuildEvent{
		Type:           builder.EventStepStarted,
		Project:        "CLIENT",
		ProjectDetails: models.Project{Name: "CLIENT", Type: models.ProjectTypeReact},
		Step:           "Install Dependencies",
		Command:        "npm install",
		StepIndex:      1,
		TotalSteps:     2,
	})
	rep.OnBuildEvent(builder.BuildEvent{
		Type:           builder.EventStepCompleted,
		Project:        "CLIENT",
		ProjectDetails: models.Project{Name: "CLIENT", Type: models.ProjectTypeReact},
		Step:           "Install Dependencies",
		Duration:       2*time.Minute + 41*time.Second,
		StepIndex:      1,
		TotalSteps:     2,
	})

	job := models.Job{
		ID:         "JOB-000123",
		Repository: "CLIENT",
		Revision:   "542",
		Status:     models.JobStatusSuccess,
		Duration:   3*time.Minute + 32*time.Second,
		StageTimings: map[string]time.Duration{
			"Checkout": 9 * time.Second,
			"Build":    42 * time.Second,
		},
		BuildResults: []models.BuildResult{
			{
				Project:  models.Project{Name: "CLIENT", Type: models.ProjectTypeReact},
				Status:   models.BuildStatusSuccess,
				Duration: 2 * time.Minute,
			},
		},
		Artifacts: []models.Artifact{{Name: "dist", Path: "/tmp/dist"}},
	}

	rep.ReportSummary(job)

	output := buf.String()

	expectedSubstrings := []string{
		"PROJECT: CLIENT",
		"Step [1/2]: Install Dependencies",
		"Command:   npm install",
		"✓ Install Dependencies (2m 41s)",
		"Build Summary",
		"Job:        JOB-000123",
		"Revision:   542",
		"Succeeded:  1",
		"Total Duration: 3m 32s",
		"Artifacts:      1",
	}

	for _, expected := range expectedSubstrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestConsoleReporterFailureReport(t *testing.T) {
	var buf bytes.Buffer
	rep := reporter.NewConsoleReporter(&buf)

	rep.ReportFailure("CLIENT", "Build", "npm run build", 1, 18*time.Second, "Type 'number' is not assignable to type 'string'")

	outStr := buf.String()

	expectedItems := []string{
		"BUILD FAILED",
		"Project:   CLIENT",
		"Step:      Build",
		"Command:   npm run build",
		"Exit Code: 1",
		"Duration:  18s",
		"Type 'number' is not assignable to type 'string'",
	}

	for _, expected := range expectedItems {
		if !strings.Contains(outStr, expected) {
			t.Errorf("expected failure output to contain %q, got:\n%s", expected, outStr)
		}
	}
}
