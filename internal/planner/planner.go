package planner

import (
	"context"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

type BuildStrategy string

const (
	BuildStrategyFailFast BuildStrategy = "fail-fast"
	BuildStrategyContinue BuildStrategy = "continue"
)

type PublishPolicy string

const (
	PublishPolicyAllOrNothing   PublishPolicy = "all-or-nothing"
	PublishPolicySuccessfulOnly PublishPolicy = "successful-only"
)

// RepositoryConfig defines source control settings.
type RepositoryConfig struct {
	Type string `yaml:"type" json:"type"`
	URL  string `yaml:"url" json:"url"`
}

// BuildSpec defines targets and execution strategy.
type BuildSpec struct {
	Strategy BuildStrategy `yaml:"strategy" json:"strategy"`
	Targets  []string      `yaml:"targets" json:"targets"`
}

// ProjectBuildSpec holds per-project build configuration options in the YAML pipeline.
type ProjectBuildSpec struct {
	// Script is the npm/pnpm/yarn script name to execute for the build step.
	// Default: "build" (npm run build). Override example: script: deploy
	Script string `yaml:"script" json:"script,omitempty"`
}

// ProjectSpec defines project metadata, workspace subpath, and target deployment path.
type ProjectSpec struct {
	Name   string           `yaml:"name" json:"name"`
	Path   string           `yaml:"path" json:"path"`
	Output string           `yaml:"output" json:"output"`
	Build  ProjectBuildSpec `yaml:"build" json:"build,omitempty"`
}

// DeploymentSpec defines publishing policy and overwrite behavior.
type DeploymentSpec struct {
	ReplaceExisting bool          `yaml:"replaceExisting" json:"replace_existing"`
	Publish         PublishPolicy `yaml:"publish" json:"publish"`
}

// PipelineConfig represents the unmarshaled YAML document schema.
type PipelineConfig struct {
	Version    int            `yaml:"version" json:"version"`
	Repository RepositoryConfig `yaml:"repository" json:"repository"`
	Build      BuildSpec      `yaml:"build" json:"build"`
	Deployment DeploymentSpec `yaml:"deployment" json:"deployment"`
	Projects   []ProjectSpec  `yaml:"projects" json:"projects"`
}

// ExecutionPlan represents the validated and resolved pipeline execution instructions.
type ExecutionPlan struct {
	Repository      RepositoryConfig
	Projects        []models.Project
	BuildStrategy   BuildStrategy
	PublishPolicy   PublishPolicy
	ReplaceExisting bool
	TargetOutputs   map[string]string // Maps project name -> target output path
}

// Planner defines interface for loading, validating, and resolving pipeline plans.
type Planner interface {
	Load(filePath string) (*PipelineConfig, error)
	Validate(cfg *PipelineConfig) error
	Resolve(ctx context.Context, cfg *PipelineConfig, scannedProjects []models.Project, workspacePath string) (*ExecutionPlan, error)
}
