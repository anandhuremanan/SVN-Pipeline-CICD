package planner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/planner"
)

const sampleYAML = `
version: 1

repository:
  type: svn
  url: https://versionvault.grampro.local/svn/Agritech

build:
  strategy: fail-fast
  targets:
    - Farmer
    - API

deployment:
  replaceExisting: true
  publish: all-or-nothing

projects:
  - name: Farmer
    path: Client/Farmer
    output: D:/Deployments/Farmer

  - name: Seller
    path: Client/Seller
    output: D:/Deployments/Seller

  - name: API
    path: API/GRAMPRO.Platform.Utility.API
    output: D:/Deployments/API
`

func TestLoadAndValidateYAML(t *testing.T) {
	tempDir := t.TempDir()
	yamlFile := filepath.Join(tempDir, "cicd.yaml")
	if err := os.WriteFile(yamlFile, []byte(sampleYAML), 0644); err != nil {
		t.Fatalf("failed to write test yaml: %v", err)
	}

	loader := planner.NewDefaultLoader()
	cfg, err := loader.Load(yamlFile)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	validator := planner.NewDefaultValidator()
	if err := validator.Validate(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	if cfg.Repository.URL != "https://versionvault.grampro.local/svn/Agritech" {
		t.Errorf("unexpected repository URL: %s", cfg.Repository.URL)
	}

	if len(cfg.Build.Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(cfg.Build.Targets))
	}
}

func TestResolveExecutionPlanSuccess(t *testing.T) {
	tempDir := t.TempDir()
	workspaceDir := filepath.Join(tempDir, "workspace")
	_ = os.MkdirAll(filepath.Join(workspaceDir, "Client/Farmer"), 0755)
	_ = os.MkdirAll(filepath.Join(workspaceDir, "API/GRAMPRO.Platform.Utility.API"), 0755)

	yamlFile := filepath.Join(tempDir, "cicd.yaml")
	_ = os.WriteFile(yamlFile, []byte(sampleYAML), 0644)

	loader := planner.NewDefaultLoader()
	cfg, _ := loader.Load(yamlFile)

	scanned := []models.Project{
		{Name: "Farmer", Path: "Client/Farmer", Type: models.ProjectTypeReact},
		{Name: "API", Path: "API/GRAMPRO.Platform.Utility.API", Type: models.ProjectTypeDotnet},
	}

	validator := planner.NewDefaultValidator()
	plan, err := validator.Resolve(context.Background(), cfg, scanned, workspaceDir)
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}

	if len(plan.Projects) != 2 {
		t.Fatalf("expected 2 resolved projects, got %d", len(plan.Projects))
	}

	if plan.TargetOutputs["Farmer"] != "D:/Deployments/Farmer" {
		t.Errorf("unexpected target output for Farmer: %s", plan.TargetOutputs["Farmer"])
	}
}

func TestResolveMissingProjectPath(t *testing.T) {
	tempDir := t.TempDir()
	workspaceDir := filepath.Join(tempDir, "workspace")
	// Only create Client/Farmer, do NOT create API directory
	_ = os.MkdirAll(filepath.Join(workspaceDir, "Client/Farmer"), 0755)

	yamlFile := filepath.Join(tempDir, "cicd.yaml")
	_ = os.WriteFile(yamlFile, []byte(sampleYAML), 0644)

	loader := planner.NewDefaultLoader()
	cfg, _ := loader.Load(yamlFile)

	validator := planner.NewDefaultValidator()
	_, err := validator.Resolve(context.Background(), cfg, nil, workspaceDir)
	if err == nil {
		t.Error("expected error for missing project path, got nil")
	}
}
