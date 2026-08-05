package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/retry"
)

func TestIsTransientStep(t *testing.T) {
	tests := []struct {
		cmd         string
		step        string
		isTransient bool
	}{
		{"svn", "Repository Checkout", true},
		{"npm", "Install Dependencies", true},
		{"dotnet", "Restore", true},
		{"npm", "Build", false},
		{"dotnet", "Build", false},
		{"tsc", "Compile", false},
	}

	for _, tt := range tests {
		t.Run(tt.cmd+"_"+tt.step, func(t *testing.T) {
			res := retry.IsTransientStep(tt.cmd, tt.step)
			if res != tt.isTransient {
				t.Errorf("expected isTransient %v, got %v for cmd=%s step=%s", tt.isTransient, res, tt.cmd, tt.step)
			}
		})
	}
}

func TestRetryExecuteSuccessOnRetry(t *testing.T) {
	policy := retry.Policy{
		Enabled:        true,
		Count:          2,
		BackoffSeconds: 1 * time.Millisecond,
	}

	attempts := 0
	op := func() error {
		attempts++
		if attempts < 2 {
			return errors.New("network error")
		}
		return nil
	}

	err := retry.Execute(context.Background(), policy, logger.NopLogger(), "npm install", true, op)
	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}

	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetryExecuteNonTransient(t *testing.T) {
	policy := retry.Policy{
		Enabled:        true,
		Count:          3,
		BackoffSeconds: 1 * time.Millisecond,
	}

	attempts := 0
	op := func() error {
		attempts++
		return errors.New("compilation error")
	}

	err := retry.Execute(context.Background(), policy, logger.NopLogger(), "npm build", false, op)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if attempts != 1 {
		t.Errorf("expected non-transient build operation to NOT retry (1 attempt), got %d", attempts)
	}
}
