package scanner

import (
	"context"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// Detector defines an extensible interface for discovering a specific project type in a directory.
type Detector interface {
	// Name returns the identifier of the detector (e.g., "React", "Next.js", ".NET").
	Name() string
	// Detect inspects a directory and returns a Project model if a match is found.
	Detect(ctx context.Context, projectDir string, relPath string, entries []string) (*models.Project, bool, error)
}
