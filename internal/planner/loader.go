package planner

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// DefaultLoader implements YAML loading and parsing.
type DefaultLoader struct{}

// NewDefaultLoader constructs a DefaultLoader instance.
func NewDefaultLoader() *DefaultLoader {
	return &DefaultLoader{}
}

// Load reads and parses a YAML pipeline configuration file.
func (l *DefaultLoader) Load(filePath string) (*PipelineConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("planner: failed to read pipeline configuration file %q: %w", filePath, err)
	}

	var cfg PipelineConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("planner: failed to parse YAML configuration %q: %w", filePath, err)
	}

	return &cfg, nil
}
