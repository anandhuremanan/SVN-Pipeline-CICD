package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/scanner"
)

func TestScanProjects(t *testing.T) {
	rootDir := t.TempDir()

	// 1. React Vite project
	reactDir := filepath.Join(rootDir, "client")
	if err := os.MkdirAll(reactDir, 0755); err != nil {
		t.Fatalf("failed to create react dir: %v", err)
	}
	reactPkg := []byte(`{
		"name": "react-client",
		"dependencies": { "react": "^18.0.0" },
		"devDependencies": { "vite": "^5.0.0" }
	}`)
	if err := os.WriteFile(filepath.Join(reactDir, "package.json"), reactPkg, 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	// 2. Next.js project
	nextDir := filepath.Join(rootDir, "admin")
	if err := os.MkdirAll(nextDir, 0755); err != nil {
		t.Fatalf("failed to create next dir: %v", err)
	}
	nextPkg := []byte(`{
		"name": "next-admin",
		"dependencies": { "next": "^14.0.0", "react": "^18.0.0" }
	}`)
	if err := os.WriteFile(filepath.Join(nextDir, "package.json"), nextPkg, 0644); err != nil {
		t.Fatalf("failed to write next package.json: %v", err)
	}

	// 3. Dotnet project
	dotnetDir := filepath.Join(rootDir, "services", "api")
	if err := os.MkdirAll(dotnetDir, 0755); err != nil {
		t.Fatalf("failed to create dotnet dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dotnetDir, "API.csproj"), []byte("<Project></Project>"), 0644); err != nil {
		t.Fatalf("failed to write csproj: %v", err)
	}

	repoScanner := scanner.NewRepositoryScanner(logger.NopLogger())
	projects, err := repoScanner.Scan(context.Background(), rootDir)

	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	if len(projects) != 3 {
		t.Fatalf("expected 3 detected projects, got %d", len(projects))
	}

	projectTypes := make(map[models.ProjectType]bool)
	for _, p := range projects {
		projectTypes[p.Type] = true
	}

	if !projectTypes[models.ProjectTypeReact] {
		t.Error("expected React project to be detected")
	}
	if !projectTypes[models.ProjectTypeNext] {
		t.Error("expected Next.js project to be detected")
	}
	if !projectTypes[models.ProjectTypeDotnet] {
		t.Error("expected .NET project to be detected")
	}
}
