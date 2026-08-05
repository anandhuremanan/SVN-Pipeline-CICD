package planner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// DefaultValidator implements configuration validation and resolution.
type DefaultValidator struct{}

// NewDefaultValidator constructs a DefaultValidator instance.
func NewDefaultValidator() *DefaultValidator {
	return &DefaultValidator{}
}

// Validate checks that the unmarshaled YAML configuration meets all schema constraints.
func (v *DefaultValidator) Validate(cfg *PipelineConfig) error {
	if cfg == nil {
		return fmt.Errorf("planner: pipeline configuration is nil")
	}

	if cfg.Version != 1 {
		return fmt.Errorf("planner: unsupported pipeline configuration version %d (expected version: 1)", cfg.Version)
	}

	if strings.TrimSpace(cfg.Repository.URL) == "" {
		return fmt.Errorf("planner: repository URL cannot be empty")
	}

	switch strings.ToLower(string(cfg.Build.Strategy)) {
	case "", string(BuildStrategyFailFast):
		cfg.Build.Strategy = BuildStrategyFailFast
	case string(BuildStrategyContinue):
		cfg.Build.Strategy = BuildStrategyContinue
	default:
		return fmt.Errorf("planner: unsupported build strategy %q (must be 'fail-fast' or 'continue')", cfg.Build.Strategy)
	}

	switch strings.ToLower(string(cfg.Deployment.Publish)) {
	case "", string(PublishPolicyAllOrNothing):
		cfg.Deployment.Publish = PublishPolicyAllOrNothing
	case string(PublishPolicySuccessfulOnly):
		cfg.Deployment.Publish = PublishPolicySuccessfulOnly
	default:
		return fmt.Errorf("planner: unsupported publish policy %q (must be 'all-or-nothing' or 'successful-only')", cfg.Deployment.Publish)
	}

	if len(cfg.Projects) == 0 {
		return fmt.Errorf("planner: projects list cannot be empty")
	}

	return nil
}

// Resolve matches configured build targets against checked-out workspace project paths and scanned detectors.
func (v *DefaultValidator) Resolve(ctx context.Context, cfg *PipelineConfig, scannedProjects []models.Project, workspacePath string) (*ExecutionPlan, error) {
	if err := v.Validate(cfg); err != nil {
		return nil, err
	}

	// Index projects defined in YAML by Name and Path
	specByName := make(map[string]ProjectSpec)
	specByPath := make(map[string]ProjectSpec)
	for _, pSpec := range cfg.Projects {
		pName := strings.TrimSpace(pSpec.Name)
		pPath := filepath.Clean(strings.TrimSpace(pSpec.Path))
		if pName != "" {
			specByName[pName] = pSpec
		}
		if pPath != "" {
			specByPath[pPath] = pSpec
		}
	}

	// Determine targets to execute
	targetNames := cfg.Build.Targets
	if len(targetNames) == 0 {
		for _, pSpec := range cfg.Projects {
			targetNames = append(targetNames, pSpec.Name)
		}
	}

	// Index scanned projects by relative path and name
	scannedByPath := make(map[string]models.Project)
	scannedByName := make(map[string]models.Project)
	for _, sp := range scannedProjects {
		rel := filepath.Clean(sp.Path)
		scannedByPath[rel] = sp
		if sp.Name != "" {
			scannedByName[sp.Name] = sp
		}
	}

	targetOutputs := make(map[string]string)
	resolvedProjects := make([]models.Project, 0, len(targetNames))

	for _, targetName := range targetNames {
		pSpec, ok := specByName[targetName]
		if !ok {
			return nil, fmt.Errorf("Pipeline Validation Failed\n\nConfigured project\n\n%s\n\nExpected Path\n\nUnknown\n\nProject not found in YAML projects definition.", targetName)
		}

		expectedPath := filepath.Join(workspacePath, pSpec.Path)
		if _, err := os.Stat(expectedPath); err != nil {
			return nil, fmt.Errorf("Pipeline Validation Failed\n\nConfigured project\n\n%s\n\nExpected Path\n\n%s\n\nProject not found.", targetName, pSpec.Path)
		}

		cleanPath := filepath.Clean(pSpec.Path)
		scanned, found := scannedByPath[cleanPath]
		if !found {
			scanned, found = scannedByName[pSpec.Name]
		}

		if !found {
			// If detector didn't pick it up, construct project from spec
			scanned = models.Project{
				Name:    pSpec.Name,
				Path:    pSpec.Path,
				Type:    models.ProjectTypeUnknown,
				RootDir: expectedPath,
				Build:   models.BuildConfig{Script: pSpec.Build.Script},
			}
		} else {
			scanned.Name = pSpec.Name
			scanned.Build = models.BuildConfig{Script: pSpec.Build.Script}
		}

		resolvedProjects = append(resolvedProjects, scanned)

		if pSpec.Output != "" {
			targetOutputs[pSpec.Name] = pSpec.Output
		}
	}

	return &ExecutionPlan{
		Repository:      cfg.Repository,
		Projects:        resolvedProjects,
		BuildStrategy:   cfg.Build.Strategy,
		PublishPolicy:   cfg.Deployment.Publish,
		ReplaceExisting: cfg.Deployment.ReplaceExisting,
		TargetOutputs:   targetOutputs,
	}, nil
}
