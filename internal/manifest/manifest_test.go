package manifest_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/manifest"
	"github.com/anandhuremanan/cicd-engine/internal/models"
)

func TestGenerateManifest(t *testing.T) {
	wsDir := t.TempDir()

	job := models.Job{
		ID:         "job-test-manifest",
		Repository: "https://svn.example.com/repo",
		Revision:   "500",
		Status:     models.JobStatusSuccess,
		StartedAt:  time.Now().Add(-1 * time.Minute),
		FinishedAt: time.Now(),
		Duration:   1 * time.Minute,
		Artifacts: []models.Artifact{
			{Name: "dist", Path: filepath.Join(wsDir, "dist")},
		},
	}

	if err := manifest.Generate(wsDir, job); err != nil {
		t.Fatalf("failed to generate manifest: %v", err)
	}

	manifestFile := filepath.Join(wsDir, "manifest.json")
	if _, err := os.Stat(manifestFile); os.IsNotExist(err) {
		t.Fatalf("expected manifest.json to be created at %s", manifestFile)
	}

	data, err := os.ReadFile(manifestFile)
	if err != nil || len(data) == 0 {
		t.Fatalf("failed to read non-empty manifest.json")
	}
}
