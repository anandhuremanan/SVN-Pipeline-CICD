package process

import (
	"context"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// MockRunner is an in-memory mock implementation of Runner for testing.
type MockRunner struct {
	RunFunc func(ctx context.Context, opts models.ExecOptions, handlers ...StreamHandler) (models.ProcessResult, error)
}

// NewMockRunner constructs a MockRunner with default success behavior.
func NewMockRunner() *MockRunner {
	return &MockRunner{
		RunFunc: func(ctx context.Context, opts models.ExecOptions, handlers ...StreamHandler) (models.ProcessResult, error) {
			return models.NewProcessResult(opts.Command, 0, "mock stdout", "", 0, nil), nil
		},
	}
}

// Run executes the registered mock function.
func (m *MockRunner) Run(ctx context.Context, opts models.ExecOptions, handlers ...StreamHandler) (models.ProcessResult, error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, opts, handlers...)
	}
	return models.NewProcessResult(opts.Command, 0, "", "", 0, nil), nil
}
