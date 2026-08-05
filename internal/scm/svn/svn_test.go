package svn_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/credentials"
	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/process"
	"github.com/anandhuremanan/cicd-engine/internal/scm/svn"
)

func TestSVNProvider(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "workspace")

	mockRunner := process.NewMockRunner()
	mockRunner.RunFunc = func(ctx context.Context, opts models.ExecOptions, handlers ...process.StreamHandler) (models.ProcessResult, error) {
		if opts.Args[0] == "checkout" {
			_ = os.MkdirAll(filepath.Join(targetDir, ".svn"), 0755)
			return models.NewProcessResult("svn", 0, "Checked out revision 100.", "", 0, nil), nil
		}
		if opts.Args[0] == "info" {
			return models.NewProcessResult("svn", 0, "100\n", "", 0, nil), nil
		}
		return models.NewProcessResult("svn", 0, "", "", 0, nil), nil
	}

	provider := svn.NewProvider(mockRunner, logger.NopLogger())
	if provider.Name() != "SVN" {
		t.Errorf("expected provider name SVN, got %s", provider.Name())
	}

	info, err := provider.CheckoutOrUpdate(context.Background(), "https://svn.example.com/repo", targetDir, "HEAD", credentials.NewStaticProvider("user", "pass"))
	if err != nil {
		t.Fatalf("unexpected checkout error: %v", err)
	}

	if info.Revision != "100" {
		t.Errorf("expected revision 100, got %s", info.Revision)
	}
}
