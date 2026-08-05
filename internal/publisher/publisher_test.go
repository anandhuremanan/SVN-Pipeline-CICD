package publisher_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/publisher"
)

func TestPublisherReactDist(t *testing.T) {
	tempDir := t.TempDir()
	artifactDir := filepath.Join(tempDir, "workspace", "dist")
	_ = os.MkdirAll(filepath.Join(artifactDir, "assets"), 0755)
	_ = os.WriteFile(filepath.Join(artifactDir, "index.html"), []byte("<html></html>"), 0644)
	_ = os.WriteFile(filepath.Join(artifactDir, "assets", "app.js"), []byte("console.log('test');"), 0644)

	deployRoot := filepath.Join(tempDir, "deployments")

	pub := publisher.NewDefaultPublisher(nil)
	proj := models.Project{Name: "CLIENT", Type: models.ProjectTypeReact}
	bRes := models.BuildResult{
		Project: proj,
		Status:  models.BuildStatusSuccess,
		Artifacts: []models.Artifact{
			{Name: "dist", Path: artifactDir},
		},
	}

	targetDir, err := pub.Publish(context.Background(), "UAM", proj, bRes, deployRoot, true)
	if err != nil {
		t.Fatalf("unexpected publish error: %v", err)
	}

	expectedTarget := filepath.Join(deployRoot, "UAM", "CLIENT")
	if targetDir != expectedTarget {
		t.Errorf("expected target %q, got %q", expectedTarget, targetDir)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "index.html")); err != nil {
		t.Errorf("expected index.html in target, got error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "assets", "app.js")); err != nil {
		t.Errorf("expected assets/app.js in target, got error: %v", err)
	}
}

func TestPublisherReplaceExistingFalse(t *testing.T) {
	tempDir := t.TempDir()
	artifactDir := filepath.Join(tempDir, "workspace", "publish")
	_ = os.MkdirAll(artifactDir, 0755)
	_ = os.WriteFile(filepath.Join(artifactDir, "app.dll"), []byte("binary"), 0644)

	deployRoot := filepath.Join(tempDir, "deployments")
	existingTarget := filepath.Join(deployRoot, "UAM", "API")
	_ = os.MkdirAll(existingTarget, 0755)

	pub := publisher.NewDefaultPublisher(nil)
	proj := models.Project{Name: "API", Type: models.ProjectTypeDotnet}
	bRes := models.BuildResult{
		Project: proj,
		Status:  models.BuildStatusSuccess,
		Artifacts: []models.Artifact{
			{Name: "publish", Path: artifactDir},
		},
	}

	_, err := pub.Publish(context.Background(), "UAM", proj, bRes, deployRoot, false)
	if err == nil {
		t.Error("expected error when replaceExisting is false and destination exists, got nil")
	}
}
