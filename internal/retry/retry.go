package retry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/logger"
)

// Policy defines configuration parameters for retrying operations.
type Policy struct {
	Enabled        bool          `json:"enabled"`
	Count          int           `json:"count"`
	BackoffSeconds time.Duration `json:"backoff_seconds"`
}

// DefaultPolicy returns a standard 3-retry policy with 1s initial backoff.
func DefaultPolicy() Policy {
	return Policy{
		Enabled:        true,
		Count:          3,
		BackoffSeconds: 1 * time.Second,
	}
}

// IsTransientStep returns true if the step command is eligible for retry.
func IsTransientStep(cmdName, stepName string) bool {
	lowerCmd := strings.ToLower(cmdName)
	lowerStep := strings.ToLower(stepName)

	if strings.Contains(lowerCmd, "svn") || strings.Contains(lowerStep, "checkout") || strings.Contains(lowerStep, "update") {
		return true
	}

	if (strings.Contains(lowerCmd, "npm") || strings.Contains(lowerCmd, "pnpm") || strings.Contains(lowerCmd, "yarn")) &&
		(strings.Contains(lowerStep, "install") || strings.Contains(lowerCmd, "install")) {
		return true
	}

	if strings.Contains(lowerCmd, "dotnet") && (strings.Contains(lowerStep, "restore") || strings.Contains(lowerCmd, "restore")) {
		return true
	}

	return false
}

// Execute runs operation and retries up to policy.Count times with exponential backoff on failure.
func Execute(ctx context.Context, policy Policy, log logger.Logger, opName string, isTransient bool, op func() error) error {
	if log == nil {
		log = logger.NopLogger()
	}

	if !policy.Enabled || policy.Count <= 0 || !isTransient {
		return op()
	}

	var lastErr error
	backoff := policy.BackoffSeconds
	if backoff <= 0 {
		backoff = 1 * time.Second
	}

	for attempt := 1; attempt <= policy.Count+1; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := op()
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt > policy.Count {
			break
		}

		log.Warn(fmt.Sprintf("transient operation %q failed (attempt %d/%d), retrying in %v...", opName, attempt, policy.Count, backoff), "err", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff *= 2
	}

	return fmt.Errorf("operation %q failed after %d retries: %w", opName, policy.Count, lastErr)
}
