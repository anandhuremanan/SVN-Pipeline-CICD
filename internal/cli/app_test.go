package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/cli"
	"github.com/anandhuremanan/cicd-engine/internal/models"
)

type MockEngine struct {
	Called bool
	Req    cli.RunRequest
}

func (m *MockEngine) Run(ctx context.Context, req cli.RunRequest) error {
	m.Called = true
	m.Req = req
	return nil
}

func (m *MockEngine) GetHistory() ([]models.Job, error) {
	return []models.Job{
		{
			ID:         "job-hist-1",
			Repository: "https://svn.example.com/repo",
			Revision:   "100",
			Status:     models.JobStatusSuccess,
			StartedAt:  time.Now().Add(-1 * time.Hour),
			Duration:   10 * time.Second,
		},
	}, nil
}

func (m *MockEngine) GetStatus() ([]models.Job, error) {
	return []models.Job{
		{
			ID:         "job-active-1",
			Repository: "https://svn.example.com/repo",
			Revision:   "101",
			Status:     models.JobStatusRunning,
		},
	}, nil
}

func TestCLIRunCommand(t *testing.T) {
	var out, errBuf bytes.Buffer
	mockEng := &MockEngine{}
	app := cli.NewApp(&out, &errBuf, mockEng)

	args := []string{"run", "https://svn.example.com/repo/trunk", "--keep-workspace", "--username", "testuser"}
	err := app.Run(context.Background(), args)

	if err != nil {
		t.Fatalf("expected CLI run to succeed, got: %v", err)
	}

	if !mockEng.Called {
		t.Error("expected pipeline engine to be called")
	}

	if mockEng.Req.RepositoryURL != "https://svn.example.com/repo/trunk" {
		t.Errorf("expected repo URL %q, got %q", "https://svn.example.com/repo/trunk", mockEng.Req.RepositoryURL)
	}
}

func TestCLIHistoryCommand(t *testing.T) {
	var out, errBuf bytes.Buffer
	mockEng := &MockEngine{}
	app := cli.NewApp(&out, &errBuf, mockEng)

	err := app.Run(context.Background(), []string{"history"})
	if err != nil {
		t.Fatalf("unexpected history command error: %v", err)
	}

	if !strings.Contains(out.String(), "job-hist-1") {
		t.Errorf("expected job-hist-1 in history output, got: %s", out.String())
	}
}

func TestCLIStatusCommand(t *testing.T) {
	var out, errBuf bytes.Buffer
	mockEng := &MockEngine{}
	app := cli.NewApp(&out, &errBuf, mockEng)

	err := app.Run(context.Background(), []string{"status"})
	if err != nil {
		t.Fatalf("unexpected status command error: %v", err)
	}

	if !strings.Contains(out.String(), "job-active-1") {
		t.Errorf("expected job-active-1 in status output, got: %s", out.String())
	}
}

func TestCLIDoctorCommand(t *testing.T) {
	var out, errBuf bytes.Buffer
	mockEng := &MockEngine{}
	app := cli.NewApp(&out, &errBuf, mockEng)

	err := app.Run(context.Background(), []string{"doctor"})

	// doctor will run diagnostics on tools, workspace, etc.
	if !strings.Contains(out.String(), "Environment") {
		t.Errorf("expected doctor output to contain Environment, got:\n%s (err: %v)", out.String(), err)
	}
}
