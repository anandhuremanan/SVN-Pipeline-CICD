package repository_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/credentials"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/process"
	"github.com/anandhuremanan/cicd-engine/internal/repository"
)

func TestSVNCheckoutReportsProgress(t *testing.T) {
	mockRunner := process.NewMockRunner()
	mockRunner.RunFunc = func(ctx context.Context, opts models.ExecOptions, handlers ...process.StreamHandler) (models.ProcessResult, error) {
		if len(opts.Args) > 0 && opts.Args[0] == "checkout" {
			if len(handlers) != 1 {
				t.Fatalf("expected the checkout to be run with a stream handler, got %d", len(handlers))
			}
			for _, line := range []string{
				`A    ws\Client`,
				`A    ws\Client\package.json`,
				`U    ws\Client\index.html`,
				` U   ws`,
				`Checked out revision 42.`,
			} {
				handlers[0].Stdout(line)
			}
		}
		return models.NewProcessResult(opts.Command, 0, "42", "", 0, nil), nil
	}

	svc := repository.NewSVNService(mockRunner, nil)

	var reported []int
	svc.SetProgress(func(files int) { reported = append(reported, files) })

	targetDir := filepath.Join(t.TempDir(), "ws")
	if _, err := svc.CheckoutOrUpdate(context.Background(), "https://svn.example.com/repo/trunk", targetDir, "HEAD", credentials.NewStaticProvider("user", "pass")); err != nil {
		t.Fatalf("unexpected checkout error: %v", err)
	}

	// Lines arrive within the same second, so only the final total is reported.
	if len(reported) != 1 || reported[0] != 3 {
		t.Errorf("expected a single final report of 3 files, got %v", reported)
	}
}
