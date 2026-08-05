package sanitizer_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/sanitizer"
)

func TestSanitizeEmptyWorkspace(t *testing.T) {
	ws := t.TempDir()
	s := sanitizer.NewSanitizer(nil, logger.NopLogger())

	res, err := s.Sanitize(context.Background(), ws)
	if err != nil {
		t.Fatalf("unexpected error sanitizing empty workspace: %v", err)
	}

	if len(res.DirectoriesRemoved) != 0 || len(res.FilesRemoved) != 0 {
		t.Errorf("expected 0 removals for empty workspace, got %+v", res)
	}
}

func TestSanitizeReactAndNextProjects(t *testing.T) {
	ws := t.TempDir()

	// React project dir
	clientDir := filepath.Join(ws, "CLIENT")
	nodeModules := filepath.Join(clientDir, "node_modules")
	dist := filepath.Join(clientDir, "dist")
	src := filepath.Join(clientDir, "src")

	_ = os.MkdirAll(nodeModules, 0755)
	_ = os.MkdirAll(dist, 0755)
	_ = os.MkdirAll(src, 0755)

	_ = os.WriteFile(filepath.Join(nodeModules, "package.json"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(dist, "index.js"), []byte("console.log('hi')"), 0644)
	_ = os.WriteFile(filepath.Join(src, "App.tsx"), []byte("export const App = () => null;"), 0644)

	s := sanitizer.NewSanitizer(nil, logger.NopLogger())
	res, err := s.Sanitize(context.Background(), ws)
	if err != nil {
		t.Fatalf("unexpected sanitization error: %v", err)
	}

	if len(res.DirectoriesRemoved) != 2 {
		t.Fatalf("expected 2 directories removed, got %d (%+v)", len(res.DirectoriesRemoved), res.DirectoriesRemoved)
	}

	if _, err := os.Stat(src); os.IsNotExist(err) {
		t.Error("expected source directory CLIENT/src to be preserved")
	}

	if _, err := os.Stat(dist); !os.IsNotExist(err) {
		t.Error("expected dist directory to be deleted")
	}
}

func TestSanitizeDotnetProject(t *testing.T) {
	ws := t.TempDir()

	apiDir := filepath.Join(ws, "API")
	binDir := filepath.Join(apiDir, "bin")
	objDir := filepath.Join(apiDir, "obj")
	controllersDir := filepath.Join(apiDir, "Controllers")

	_ = os.MkdirAll(binDir, 0755)
	_ = os.MkdirAll(objDir, 0755)
	_ = os.MkdirAll(controllersDir, 0755)

	_ = os.WriteFile(filepath.Join(controllersDir, "TestController.cs"), []byte("// controller"), 0644)

	s := sanitizer.NewSanitizer(nil, logger.NopLogger())
	res, err := s.Sanitize(context.Background(), ws)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.DirectoriesRemoved) != 2 {
		t.Fatalf("expected 2 directories removed (bin, obj), got %d", len(res.DirectoriesRemoved))
	}

	if _, err := os.Stat(controllersDir); os.IsNotExist(err) {
		t.Error("expected Controllers directory to be preserved")
	}
}

func TestProtectedDirectoriesProtection(t *testing.T) {
	ws := t.TempDir()

	svnDir := filepath.Join(ws, ".svn")
	gitDir := filepath.Join(ws, ".git")

	_ = os.MkdirAll(svnDir, 0755)
	_ = os.MkdirAll(gitDir, 0755)

	_ = os.WriteFile(filepath.Join(svnDir, "entries"), []byte("svn info"), 0644)

	// Custom rule that attempts to target .svn
	customRules := []sanitizer.Rule{
		{Name: "SVN Attempt", Pattern: ".svn", Type: sanitizer.TypeDirectory},
		{Name: "Git Attempt", Pattern: ".git", Type: sanitizer.TypeDirectory},
	}

	s := sanitizer.NewSanitizer(customRules, logger.NopLogger())
	res, err := s.Sanitize(context.Background(), ws)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.DirectoriesRemoved) != 0 {
		t.Errorf("protected directories must never be removed, got removals: %+v", res.DirectoriesRemoved)
	}

	if _, err := os.Stat(svnDir); os.IsNotExist(err) {
		t.Error("expected .svn directory to be protected and preserved")
	}
}
