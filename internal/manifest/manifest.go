package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// Manifest defines the schema for manifest.json generated inside workspace root.
type Manifest struct {
	JobID        string               `json:"jobId"`
	Repository   string               `json:"repository"`
	Revision     string               `json:"revision"`
	Status       string               `json:"status"`
	Duration     string               `json:"duration"`
	StartedAt    string               `json:"startedAt"`
	FinishedAt   string               `json:"finishedAt"`
	Projects     []models.BuildResult `json:"projects"`
	Artifacts    []models.Artifact    `json:"artifacts"`
	ErrorMessage string               `json:"errorMessage,omitempty"`
}

// Generate creates and writes manifest.json inside workspaceDir.
func Generate(workspaceDir string, job models.Job) error {
	if workspaceDir == "" {
		return fmt.Errorf("manifest: workspaceDir cannot be empty")
	}

	m := Manifest{
		JobID:        job.ID,
		Repository:   job.Repository,
		Revision:     job.Revision,
		Status:       string(job.Status),
		Duration:     job.Duration.String(),
		StartedAt:    job.StartedAt.Format(time.RFC3339),
		FinishedAt:   job.FinishedAt.Format(time.RFC3339),
		Projects:     job.BuildResults,
		Artifacts:    job.Artifacts,
		ErrorMessage: job.ErrorMessage,
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("manifest: failed to marshal manifest JSON: %w", err)
	}

	manifestPath := filepath.Join(workspaceDir, "manifest.json")
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("manifest: failed to write manifest.json at %s: %w", manifestPath, err)
	}

	return nil
}
