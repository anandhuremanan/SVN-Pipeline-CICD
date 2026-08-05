package environment_test

import (
	"context"
	"errors"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/environment"
	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/process"
)

func TestEnvironmentValidatorSuccess(t *testing.T) {
	mockRunner := process.NewMockRunner()
	mockRunner.RunFunc = func(ctx context.Context, opts models.ExecOptions, handlers ...process.StreamHandler) (models.ProcessResult, error) {
		switch opts.Command {
		case "svn":
			return models.NewProcessResult("svn", 0, "1.14.2\n", "", 0, nil), nil
		case "node":
			return models.NewProcessResult("node", 0, "v22.15.0\n", "", 0, nil), nil
		case "npm":
			return models.NewProcessResult("npm", 0, "10.9.0\n", "", 0, nil), nil
		case "dotnet":
			return models.NewProcessResult("dotnet", 0, "9.0.100\n", "", 0, nil), nil
		default:
			return models.NewProcessResult(opts.Command, 1, "", "unknown command", 0, errors.New("not found")), errors.New("not found")
		}
	}

	validator := environment.NewValidator(mockRunner, logger.NopLogger())
	info, err := validator.Validate(context.Background())
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	svnInfo, ok := info.Tools["svn"]
	if !ok || !svnInfo.Installed || svnInfo.Version != "1.14.2" {
		t.Errorf("expected svn version 1.14.2, got %+v", svnInfo)
	}

	nodeInfo, ok := info.Tools["node"]
	if !ok || !nodeInfo.Installed || nodeInfo.Version != "v22.15.0" {
		t.Errorf("expected node version v22.15.0, got %+v", nodeInfo)
	}
}

func TestEnvironmentValidatorMissingTool(t *testing.T) {
	mockRunner := process.NewMockRunner()
	mockRunner.RunFunc = func(ctx context.Context, opts models.ExecOptions, handlers ...process.StreamHandler) (models.ProcessResult, error) {
		if opts.Command == "dotnet" {
			return models.NewProcessResult("dotnet", 1, "", "command not found", 0, errors.New("not installed")), errors.New("not installed")
		}
		return models.NewProcessResult(opts.Command, 0, "1.0.0", "", 0, nil), nil
	}

	validator := environment.NewValidator(mockRunner, logger.NopLogger())
	info, err := validator.Validate(context.Background())
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	dotnetInfo := info.Tools["dotnet"]
	if dotnetInfo.Installed {
		t.Error("expected dotnet to be marked as not installed")
	}
	if dotnetInfo.ErrorMessage == "" {
		t.Error("expected error message for missing tool")
	}
}
