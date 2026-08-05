package workspace_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/workspace"
)

func TestCreateAndCleanupWorkspace(t *testing.T) {
	tempRoot := t.TempDir()
	log := logger.NopLogger()

	mgr, err := workspace.NewManager(tempRoot, false, log)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	ws, err := mgr.CreateWorkspace("testrun")
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	if ws.Path == "" {
		t.Fatal("expected non-empty workspace path")
	}

	if _, err := os.Stat(ws.Path); os.IsNotExist(err) {
		t.Fatalf("expected workspace directory to exist at %s", ws.Path)
	}

	// Test cleanup
	if err := mgr.Cleanup(ws); err != nil {
		t.Fatalf("failed to cleanup workspace: %v", err)
	}

	if _, err := os.Stat(ws.Path); !os.IsNotExist(err) {
		t.Fatalf("expected workspace directory to be deleted after cleanup")
	}
}

func TestKeepWorkspaceOption(t *testing.T) {
	tempRoot := t.TempDir()
	log := logger.NopLogger()

	mgr, err := workspace.NewManager(tempRoot, true, log)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	ws, err := mgr.CreateWorkspace("keeprun")
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	// Create dummy file inside workspace
	dummyFile := filepath.Join(ws.Path, "dummy.txt")
	if err := os.WriteFile(dummyFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write dummy file: %v", err)
	}

	// Cleanup should be skipped because keepWorkspace is true
	if err := mgr.Cleanup(ws); err != nil {
		t.Fatalf("cleanup returned unexpected error: %v", err)
	}

	if _, err := os.Stat(dummyFile); os.IsNotExist(err) {
		t.Fatalf("expected workspace directory and dummy file to be preserved")
	}
}
