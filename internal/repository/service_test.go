package repository_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/credentials"
	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/process"
	"github.com/anandhuremanan/cicd-engine/internal/repository"
)

func TestSVNCheckoutSuccess(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "workspace")

	mockRunner := process.NewMockRunner()
	mockRunner.RunFunc = func(ctx context.Context, opts models.ExecOptions, handlers ...process.StreamHandler) (models.ProcessResult, error) {
		if opts.Args[0] == "checkout" {
			// Simulate svn creating working copy
			_ = os.MkdirAll(filepath.Join(targetDir, ".svn"), 0755)
			return models.NewProcessResult("svn", 0, "Checked out revision 123.", "", 0, nil), nil
		}
		if opts.Args[0] == "info" {
			return models.NewProcessResult("svn", 0, "123\n", "", 0, nil), nil
		}
		return models.NewProcessResult("svn", 0, "", "", 0, nil), nil
	}

	svc := repository.NewSVNService(mockRunner, logger.NopLogger())
	creds := credentials.NewStaticProvider("svnuser", "secretpass")

	info, err := svc.CheckoutOrUpdate(context.Background(), "https://svn.example.com/repo/trunk", targetDir, "HEAD", creds)
	if err != nil {
		t.Fatalf("unexpected checkout error: %v", err)
	}

	if info.Revision != "123" {
		t.Errorf("expected revision 123, got %s", info.Revision)
	}
	if info.TargetDir != targetDir {
		t.Errorf("expected target dir %s, got %s", targetDir, info.TargetDir)
	}
}

func TestSVNUpdateExistingCopy(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "workspace")
	_ = os.MkdirAll(filepath.Join(targetDir, ".svn"), 0755)

	updatedCalled := false
	mockRunner := process.NewMockRunner()
	mockRunner.RunFunc = func(ctx context.Context, opts models.ExecOptions, handlers ...process.StreamHandler) (models.ProcessResult, error) {
		if opts.Args[0] == "update" {
			updatedCalled = true
			return models.NewProcessResult("svn", 0, "Updated to revision 456.", "", 0, nil), nil
		}
		if opts.Args[0] == "info" {
			return models.NewProcessResult("svn", 0, "456\n", "", 0, nil), nil
		}
		return models.NewProcessResult("svn", 0, "", "", 0, nil), nil
	}

	svc := repository.NewSVNService(mockRunner, logger.NopLogger())
	creds := credentials.NewStaticProvider("svnuser", "secretpass")

	info, err := svc.CheckoutOrUpdate(context.Background(), "https://svn.example.com/repo/trunk", targetDir, "456", creds)
	if err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	if !updatedCalled {
		t.Error("expected svn update to be called for existing working copy")
	}
	if info.Revision != "456" {
		t.Errorf("expected revision 456, got %s", info.Revision)
	}
}
