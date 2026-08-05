package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DeploymentConfig holds configuration settings for publishing application outputs.
type DeploymentConfig struct {
	Directory       string `json:"directory"`
	ReplaceExisting bool   `json:"replace_existing"`
}

// WorkspaceConfig holds workspace directory root and cleanup policies.
type WorkspaceConfig struct {
	Root    string `json:"root"`
	Cleanup bool   `json:"cleanup"`
}

// RetryConfig holds policy settings for retrying transient operations.
type RetryConfig struct {
	Enabled        bool          `json:"enabled"`
	Count          int           `json:"count"`
	BackoffSeconds time.Duration `json:"backoff_seconds"`
}

// CleanupConfig holds workspace sanitization settings.
type CleanupConfig struct {
	Enabled bool     `json:"enabled"`
	Rules   []string `json:"rules,omitempty"`
}

// LoggingConfig holds structured logging configuration.
type LoggingConfig struct {
	Level string `json:"level"`
}

// Config represents runtime configuration settings for the CI/CD engine.
type Config struct {
	// WorkspaceRoot is the parent directory where build workspaces are generated.
	WorkspaceRoot string `json:"workspace_root"`
	// LogLevel specifies logging severity ("DEBUG", "INFO", "WARN", "ERROR").
	LogLevel string `json:"log_level"`
	// KeepWorkspace specifies whether workspaces should be preserved after pipeline runs.
	KeepWorkspace bool `json:"keep_workspace"`
	// Username is the default SVN/repository username.
	Username string `json:"username"`
	// CredentialProvider specifies how credentials are resolved ("env", "static", "wincred").
	CredentialProvider string `json:"credential_provider"`
	// ParallelBuildCount specifies the max concurrency limit for parallel project builds.
	ParallelBuildCount int `json:"parallel_build_count"`
	// Workspace holds workspace retention settings.
	Workspace WorkspaceConfig `json:"workspace"`
	// Deployment holds settings for permanent output publishing.
	Deployment DeploymentConfig `json:"deployment"`
	// Cleanup holds workspace sanitization rules.
	Cleanup CleanupConfig `json:"cleanup"`
	// Retry holds policy for retrying transient commands.
	Retry RetryConfig `json:"retry"`
	// Logging holds logging level configuration.
	Logging LoggingConfig `json:"logging"`
}

// DefaultConfig returns a Config populated with sensible default settings.
func DefaultConfig() *Config {
	return &Config{
		WorkspaceRoot:      filepath.Join(".", ".workspaces"),
		LogLevel:           "INFO",
		KeepWorkspace:      true,
		Username:           "",
		CredentialProvider: "env",
		ParallelBuildCount: 1,
		Workspace: WorkspaceConfig{
			Root:    filepath.Join(".", ".workspaces"),
			Cleanup: false,
		},
		Deployment: DeploymentConfig{
			Directory:       filepath.Join(".", ".deployments"),
			ReplaceExisting: true,
		},
		Cleanup: CleanupConfig{
			Enabled: true,
			Rules:   []string{},
		},
		Retry: RetryConfig{
			Enabled:        true,
			Count:          3,
			BackoffSeconds: 1,
		},
		Logging: LoggingConfig{
			Level: "INFO",
		},
	}
}

// Load loads configuration from a JSON file. If file path is empty, DefaultConfig is returned.
func Load(filePath string) (*Config, error) {
	cfg := DefaultConfig()

	if filePath == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: %w", filePath, err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse JSON configuration file %q: %w", filePath, err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse JSON configuration file %q: %w", filePath, err)
	}

	if wsMap, ok := raw["workspace"].(map[string]interface{}); ok {
		if root, ok := wsMap["root"].(string); ok && root != "" {
			cfg.WorkspaceRoot = root
			cfg.Workspace.Root = root
		}
		if cleanup, ok := wsMap["cleanup"].(bool); ok {
			cfg.Workspace.Cleanup = cleanup
			cfg.KeepWorkspace = !cleanup
		}
	} else if kw, ok := raw["keep_workspace"].(bool); ok {
		cfg.KeepWorkspace = kw
		cfg.Workspace.Cleanup = !kw
	}

	if depMap, ok := raw["deployment"].(map[string]interface{}); ok {
		if dir, ok := depMap["directory"].(string); ok && dir != "" {
			cfg.Deployment.Directory = dir
		}
		if repl, ok := depMap["replace_existing"].(bool); ok {
			cfg.Deployment.ReplaceExisting = repl
		} else if repl, ok := depMap["replaceExisting"].(bool); ok {
			cfg.Deployment.ReplaceExisting = repl
		}
	}

	if loggingMap, ok := raw["logging"].(map[string]interface{}); ok {
		if lvl, ok := loggingMap["level"].(string); ok && lvl != "" {
			cfg.LogLevel = lvl
			cfg.Logging.Level = lvl
		}
	} else if lvl, ok := raw["log_level"].(string); ok && lvl != "" {
		cfg.LogLevel = lvl
		cfg.Logging.Level = lvl
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration in %q: %w", filePath, err)
	}

	return cfg, nil
}

// Validate verifies that config fields contain acceptable values.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.WorkspaceRoot) == "" && strings.TrimSpace(c.Workspace.Root) == "" {
		return fmt.Errorf("workspace root cannot be empty")
	}
	if c.ParallelBuildCount < 1 {
		return fmt.Errorf("parallel_build_count must be at least 1")
	}
	if c.Retry.Enabled && c.Retry.Count < 0 {
		return fmt.Errorf("retry.count cannot be negative")
	}
	switch strings.ToLower(strings.TrimSpace(c.CredentialProvider)) {
	case "env", "static", "wincred":
		// supported providers
	default:
		return fmt.Errorf("unsupported credential_provider: %q (must be 'env', 'static', or 'wincred')", c.CredentialProvider)
	}
	return nil
}
