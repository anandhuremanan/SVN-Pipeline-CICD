package models

import "fmt"

// ProjectType represents the category of project detected in the repository.
type ProjectType string

const (
	// ProjectTypeReact represents a React application (e.g., built with Vite).
	ProjectTypeReact ProjectType = "react"
	// ProjectTypeNext represents a Next.js application.
	ProjectTypeNext ProjectType = "next"
	// ProjectTypeDotnet represents a .NET application or solution.
	ProjectTypeDotnet ProjectType = "dotnet"
	// ProjectTypeUnknown represents an unsupported or unknown project type.
	ProjectTypeUnknown ProjectType = "unknown"
)

// String returns the string representation of ProjectType.
func (pt ProjectType) String() string {
	return string(pt)
}

// IsValid checks whether the ProjectType is one of the supported types.
func (pt ProjectType) IsValid() bool {
	switch pt {
	case ProjectTypeReact, ProjectTypeNext, ProjectTypeDotnet:
		return true
	default:
		return false
	}
}

// BuildConfig holds project-level build configuration declared in cicd.yaml.
type BuildConfig struct {
	// Script is the npm/pnpm/yarn run script name to execute for the build step.
	// Defaults to "build" (i.e. npm run build) when not specified.
	Script string `yaml:"script" json:"script,omitempty"`
}

// Project contains metadata about a detected project within the repository.
type Project struct {
	// Name is the display name or directory name of the project.
	Name string `json:"name"`
	// Path is the directory path of the project relative to the workspace root.
	Path string `json:"path"`
	// Type indicates the technology stack of the project.
	Type ProjectType `json:"type"`
	// RootDir is the absolute path to the project root inside the workspace.
	RootDir string `json:"root_dir"`
	// Build holds project-specific build configuration, e.g. script override.
	Build BuildConfig `json:"build,omitempty"`
}

// Validate checks if the Project model contains valid required fields.
func (p Project) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if p.Path == "" {
		return fmt.Errorf("project path cannot be empty")
	}
	if !p.Type.IsValid() {
		return fmt.Errorf("unsupported or invalid project type: %q", p.Type)
	}
	return nil
}
