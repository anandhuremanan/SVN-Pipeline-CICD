package models_test

import (
	"errors"
	"testing"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

func TestProjectType(t *testing.T) {
	tests := []struct {
		name     string
		pt       models.ProjectType
		expected string
		isValid  bool
	}{
		{"React Project", models.ProjectTypeReact, "react", true},
		{"Next Project", models.ProjectTypeNext, "next", true},
		{"Dotnet Project", models.ProjectTypeDotnet, "dotnet", true},
		{"Unknown Project", models.ProjectTypeUnknown, "unknown", false},
		{"Custom Invalid Project", models.ProjectType("python"), "python", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pt.String() != tt.expected {
				t.Errorf("expected string %q, got %q", tt.expected, tt.pt.String())
			}
			if tt.pt.IsValid() != tt.isValid {
				t.Errorf("expected isValid %v, got %v", tt.isValid, tt.pt.IsValid())
			}
		})
	}
}

func TestProjectValidation(t *testing.T) {
	validProject := models.Project{
		Name:    "my-app",
		Path:    "apps/my-app",
		Type:    models.ProjectTypeReact,
		RootDir: "/workspace/apps/my-app",
	}

	if err := validProject.Validate(); err != nil {
		t.Fatalf("expected valid project, got error: %v", err)
	}

	invalidName := validProject
	invalidName.Name = ""
	if err := invalidName.Validate(); err == nil {
		t.Error("expected error for empty name, got nil")
	}

	invalidPath := validProject
	invalidPath.Path = ""
	if err := invalidPath.Validate(); err == nil {
		t.Error("expected error for empty path, got nil")
	}

	invalidType := validProject
	invalidType.Type = models.ProjectTypeUnknown
	if err := invalidType.Validate(); err == nil {
		t.Error("expected error for unknown project type, got nil")
	}
}

func TestProcessResult(t *testing.T) {
	resSuccess := models.NewProcessResult("npm", 0, "Build succeeded", "", 2*time.Second, nil)
	if !resSuccess.Successful {
		t.Errorf("expected process result to be successful, got %v", resSuccess.Successful)
	}
	if resSuccess.ErrorMessage != "" {
		t.Errorf("expected empty error message, got %q", resSuccess.ErrorMessage)
	}

	resFailed := models.NewProcessResult("dotnet", 1, "", "Build failed", 1*time.Second, errors.New("exit status 1"))
	if resFailed.Successful {
		t.Errorf("expected process result to fail, got successful = %v", resFailed.Successful)
	}
	if resFailed.ErrorMessage != "exit status 1" {
		t.Errorf("expected error message %q, got %q", "exit status 1", resFailed.ErrorMessage)
	}
}

func TestSerializableModelsErrorMessage(t *testing.T) {
	buildRes := models.BuildResult{
		Project: models.Project{
			Name: "web",
			Path: "web",
			Type: models.ProjectTypeNext,
		},
		Status:       models.BuildStatusFailed,
		Duration:     3 * time.Second,
		ErrorMessage: "compilation error",
	}
	if buildRes.ErrorMessage != "compilation error" {
		t.Errorf("expected error message %q, got %q", "compilation error", buildRes.ErrorMessage)
	}

	stageRes := models.StageResult{
		Name:         "Scan",
		Status:       models.StageStatusFailed,
		Duration:     500 * time.Millisecond,
		ErrorMessage: "access denied",
	}
	if stageRes.ErrorMessage != "access denied" {
		t.Errorf("expected error message %q, got %q", "access denied", stageRes.ErrorMessage)
	}

	pipelineRes := models.PipelineResult{
		Success:      false,
		Duration:     4 * time.Second,
		StageResults: []models.StageResult{stageRes},
		BuildResults: []models.BuildResult{buildRes},
		ErrorMessage: "pipeline halted",
	}
	if pipelineRes.ErrorMessage != "pipeline halted" {
		t.Errorf("expected error message %q, got %q", "pipeline halted", pipelineRes.ErrorMessage)
	}
}
