package repository_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/credentials"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/process"
	"github.com/anandhuremanan/cicd-engine/internal/repository"
)

// folderRunner records svn commands and creates the folders that exist in its pretend repository.
func folderRunner(commands *[]string, existing ...string) *process.MockRunner {
	runner := process.NewMockRunner()
	runner.RunFunc = func(ctx context.Context, opts models.ExecOptions, handlers ...process.StreamHandler) (models.ProcessResult, error) {
		*commands = append(*commands, strings.Join(opts.Args, " "))
		if opts.Args[0] == "update" && opts.Args[1] == "--parents" {
			target := opts.Args[4]
			for _, folder := range existing {
				if strings.HasSuffix(filepath.ToSlash(target), "/"+folder) {
					_ = os.MkdirAll(target, 0755)
				}
			}
		}
		return models.NewProcessResult(opts.Command, 0, "42", "", 0, nil), nil
	}
	return runner
}

func TestCheckoutFoldersDownloadsOnlyRequestedFolders(t *testing.T) {
	var commands []string
	svc := repository.NewSVNService(folderRunner(&commands, "Client", "Services/Api"), nil)
	targetDir := filepath.Join(t.TempDir(), "ws")

	info, err := svc.CheckoutFolders(context.Background(), "https://svn.example.com/repo/trunk", targetDir, "HEAD",
		[]string{"Client", `Services\Api`}, credentials.NewStaticProvider("user", "pass"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Revision != "42" {
		t.Errorf("expected revision 42, got %q", info.Revision)
	}

	joined := strings.Join(commands, "\n")
	if !strings.Contains(commands[0], "checkout https://svn.example.com/repo/trunk") || !strings.Contains(commands[0], "--depth files") {
		t.Errorf("expected a top-folder-only checkout first, got: %s", commands[0])
	}
	for _, folder := range []string{"Client", filepath.Join("Services", "Api")} {
		want := "update --parents --set-depth infinity " + filepath.Join(targetDir, folder)
		if !strings.Contains(joined, want) {
			t.Errorf("expected command %q, commands were:\n%s", want, joined)
		}
	}
	// Folder updates are pinned to the revision of the first checkout.
	if strings.Count(joined, "--revision 42") != 2 {
		t.Errorf("expected both folder updates to be pinned to revision 42, commands were:\n%s", joined)
	}
	if strings.Contains(joined, "update --set-depth infinity "+targetDir) {
		t.Errorf("did not expect the whole tree to be downloaded, commands were:\n%s", joined)
	}
}

func TestCheckoutFoldersFallsBackToEverythingForUnknownFolder(t *testing.T) {
	for _, folders := range [][]string{{"Client", "Clinet"}, {"."}, {`..\outside`}} {
		var commands []string
		svc := repository.NewSVNService(folderRunner(&commands, "Client"), nil)
		targetDir := filepath.Join(t.TempDir(), "ws")

		if _, err := svc.CheckoutFolders(context.Background(), "https://svn.example.com/repo/trunk", targetDir, "HEAD", folders, nil); err != nil {
			t.Fatalf("folders %v: unexpected error: %v", folders, err)
		}

		joined := strings.Join(commands, "\n")
		if !strings.Contains(joined, "update --set-depth infinity "+targetDir) {
			t.Errorf("folders %v: expected the whole tree to be downloaded, commands were:\n%s", folders, joined)
		}
		if strings.Contains(joined, "outside") {
			t.Errorf("folders %v: a path outside the working copy reached svn:\n%s", folders, joined)
		}
	}
}
