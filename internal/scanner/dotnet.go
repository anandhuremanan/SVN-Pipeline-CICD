package scanner

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// DotnetDetector discovers .NET C# project (*.csproj) or solution (*.sln) files.
type DotnetDetector struct{}

// NewDotnetDetector constructs a DotnetDetector instance.
func NewDotnetDetector() *DotnetDetector {
	return &DotnetDetector{}
}

func (d *DotnetDetector) Name() string {
	return ".NET"
}

func (d *DotnetDetector) Detect(ctx context.Context, projectDir string, relPath string, entries []string) (*models.Project, bool, error) {
	var projectFile string
	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry))
		if ext == ".csproj" || ext == ".sln" {
			projectFile = entry
			break
		}
	}

	if projectFile == "" {
		return nil, false, nil
	}

	name := strings.TrimSuffix(projectFile, filepath.Ext(projectFile))
	if name == "" {
		name = filepath.Base(projectDir)
	}

	return &models.Project{
		Name:    name,
		Path:    relPath,
		Type:    models.ProjectTypeDotnet,
		RootDir: projectDir,
	}, true, nil
}
