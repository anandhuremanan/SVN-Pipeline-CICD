package process_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/process"
)

type testStreamHandler struct {
	mu     sync.Mutex
	stdout []string
	stderr []string
}

func (t *testStreamHandler) Stdout(line string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stdout = append(t.stdout, line)
}

func (t *testStreamHandler) Stderr(line string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stderr = append(t.stderr, line)
}

func TestNativeRunnerSuccess(t *testing.T) {
	runner := process.NewNativeRunner()
	opts := models.ExecOptions{
		Command: "go",
		Args:    []string{"version"},
	}

	handler := &testStreamHandler{}
	result, err := runner.Run(context.Background(), opts, handler)
	if err != nil {
		t.Fatalf("expected command execution to succeed, got error: %v", err)
	}

	if !result.Successful {
		t.Errorf("expected result.Successful to be true, got false")
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	if !strings.Contains(result.StdOut, "go version") {
		t.Errorf("expected 'go version' in stdout, got: %s", result.StdOut)
	}

	if len(handler.stdout) == 0 {
		t.Errorf("expected streamed stdout lines, got 0 lines")
	}
}

func TestNativeRunnerTimeout(t *testing.T) {
	runner := process.NewNativeRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	opts := models.ExecOptions{
		Command: "go",
		Args:    []string{"test", "./..."},
		Timeout: 1 * time.Millisecond,
	}

	_, err := runner.Run(ctx, opts)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestMockRunner(t *testing.T) {
	mock := process.NewMockRunner()
	mock.RunFunc = func(ctx context.Context, opts models.ExecOptions, handlers ...process.StreamHandler) (models.ProcessResult, error) {
		return models.NewProcessResult(opts.Command, 0, "custom output", "", 5*time.Millisecond, nil), nil
	}

	opts := models.ExecOptions{Command: "npm", Args: []string{"run", "build"}}
	res, err := mock.Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected mock error: %v", err)
	}

	if res.StdOut != "custom output" {
		t.Errorf("expected custom output, got %s", res.StdOut)
	}
}
