package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.LogLevel != "INFO" {
		t.Errorf("expected default log level INFO, got %s", cfg.LogLevel)
	}
	if cfg.ParallelBuildCount != 1 {
		t.Errorf("expected default parallel build count 1, got %d", cfg.ParallelBuildCount)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid default config, got error: %v", err)
	}
}

func TestLoadConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.json")

	content := []byte(`{
		"workspace_root": "/custom/workspace",
		"log_level": "DEBUG",
		"keep_workspace": true,
		"username": "testuser",
		"credential_provider": "static",
		"parallel_build_count": 4
	}`)

	if err := os.WriteFile(configFile, content, 0644); err != nil {
		t.Fatalf("failed to write test config file: %v", err)
	}

	cfg, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("failed to load valid config: %v", err)
	}

	if cfg.WorkspaceRoot != "/custom/workspace" {
		t.Errorf("expected workspace root /custom/workspace, got %s", cfg.WorkspaceRoot)
	}
	if cfg.LogLevel != "DEBUG" {
		t.Errorf("expected log level DEBUG, got %s", cfg.LogLevel)
	}
	if !cfg.KeepWorkspace {
		t.Error("expected KeepWorkspace to be true")
	}
	if cfg.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", cfg.Username)
	}
	if cfg.CredentialProvider != "static" {
		t.Errorf("expected credential provider static, got %s", cfg.CredentialProvider)
	}
	if cfg.ParallelBuildCount != 4 {
		t.Errorf("expected parallel build count 4, got %d", cfg.ParallelBuildCount)
	}
}

func TestInvalidConfigValidation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ParallelBuildCount = 0
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for ParallelBuildCount < 1, got nil")
	}

	cfg = config.DefaultConfig()
	cfg.CredentialProvider = "invalid_provider"
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for invalid CredentialProvider, got nil")
	}
}
