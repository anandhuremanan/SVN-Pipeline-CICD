package environment

import (
	"context"
	"fmt"
	"strings"

	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/process"
)

// ToolInfo holds metadata and status for an external CLI dependency.
type ToolInfo struct {
	Name         string `json:"name"`
	Installed    bool   `json:"installed"`
	Version      string `json:"version"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// EnvironmentInfo summarizes external tools detected in the execution environment.
type EnvironmentInfo struct {
	Tools map[string]ToolInfo `json:"tools"`
}

// Validator defines operations to inspect host environment tools.
type Validator interface {
	// Validate checks availability and versions of required system binaries.
	Validate(ctx context.Context) (EnvironmentInfo, error)
}

// DefaultValidator implements Validator using Process Runner.
type DefaultValidator struct {
	runner process.Runner
	logger logger.Logger
}

// NewValidator constructs a DefaultValidator with injected dependencies.
func NewValidator(runner process.Runner, log logger.Logger) *DefaultValidator {
	if log == nil {
		log = logger.NopLogger()
	}
	return &DefaultValidator{
		runner: runner,
		logger: log,
	}
}

// Validate executes tool checks for svn, node, npm, and dotnet.
func (v *DefaultValidator) Validate(ctx context.Context) (EnvironmentInfo, error) {
	v.logger.Info("validating execution environment tools")

	tools := make(map[string]ToolInfo)

	toolConfigs := []struct {
		name string
		cmd  string
		args []string
	}{
		{"SVN", "svn", []string{"--version", "--quiet"}},
		{"Node", "node", []string{"--version"}},
		{"NPM", "npm", []string{"--version"}},
		{"Dotnet", "dotnet", []string{"--version"}},
	}

	for _, tc := range toolConfigs {
		info := v.checkTool(ctx, tc.name, tc.cmd, tc.args)
		tools[strings.ToLower(tc.name)] = info

		if info.Installed {
			v.logger.Info(fmt.Sprintf("found tool %s", tc.name), "version", info.Version)
		} else {
			v.logger.Warn(fmt.Sprintf("tool %s is not installed or available", tc.name), "err", info.ErrorMessage)
		}
	}

	return EnvironmentInfo{Tools: tools}, nil
}

func (v *DefaultValidator) checkTool(ctx context.Context, name, command string, args []string) ToolInfo {
	opts := models.ExecOptions{
		Command: command,
		Args:    args,
	}

	res, err := v.runner.Run(ctx, opts)
	if err != nil || !res.Successful {
		errMsg := res.ErrorMessage
		if errMsg == "" && err != nil {
			errMsg = err.Error()
		}
		return ToolInfo{
			Name:         name,
			Installed:    false,
			ErrorMessage: errMsg,
		}
	}

	version := strings.TrimSpace(res.StdOut)
	if idx := strings.Index(version, "\n"); idx != -1 {
		version = strings.TrimSpace(version[:idx])
	}

	return ToolInfo{
		Name:      name,
		Installed: true,
		Version:   version,
	}
}
