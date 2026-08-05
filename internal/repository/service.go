package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/credentials"
	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/process"
)

// Info contains metadata about a checked-out repository workspace.
type Info struct {
	// URL is the SVN repository remote URL.
	URL string `json:"url"`
	// Revision is the checked-out SVN revision.
	Revision string `json:"revision"`
	// TargetDir is the local absolute workspace path.
	TargetDir string `json:"target_dir"`
	// CheckedOutAt records when checkout or update completed.
	CheckedOutAt time.Time `json:"checked_out_at"`
}

// Service defines operations for repository checkout, update, and revision queries.
type Service interface {
	// CheckoutOrUpdate checks out or updates the working copy at targetDir.
	CheckoutOrUpdate(ctx context.Context, repoURL, targetDir, revision string, creds credentials.Provider) (*Info, error)
	// GetRevision queries the current SVN revision of an existing workspace.
	GetRevision(ctx context.Context, targetDir string) (string, error)
}

// SVNService implements Service using native SVN process execution.
type SVNService struct {
	runner process.Runner
	logger logger.Logger
}

// NewSVNService creates a new SVNService with injected dependencies.
func NewSVNService(runner process.Runner, log logger.Logger) *SVNService {
	if log == nil {
		log = logger.NopLogger()
	}
	return &SVNService{
		runner: runner,
		logger: log,
	}
}

// CheckoutOrUpdate checks out the repository or updates an existing checkout directory.
func (s *SVNService) CheckoutOrUpdate(ctx context.Context, repoURL, targetDir, revision string, creds credentials.Provider) (*Info, error) {
	if repoURL == "" {
		return nil, fmt.Errorf("repository service: repoURL cannot be empty")
	}
	if targetDir == "" {
		return nil, fmt.Errorf("repository service: targetDir cannot be empty")
	}
	if revision == "" {
		revision = "HEAD"
	}

	svnDir := filepath.Join(targetDir, ".svn")
	_, err := os.Stat(svnDir)
	isWorkingCopy := err == nil

	var actionErr error
	if isWorkingCopy {
		s.logger.Info("updating existing SVN working copy", "url", repoURL, "dir", targetDir)
		actionErr = s.doUpdate(ctx, targetDir, revision, creds)
	} else {
		s.logger.Info("checking out SVN repository", "url", repoURL, "dir", targetDir)
		actionErr = s.doCheckout(ctx, repoURL, targetDir, revision, creds)
	}

	if actionErr != nil {
		return nil, actionErr
	}

	currentRev, err := s.GetRevision(ctx, targetDir)
	if err != nil {
		currentRev = revision
	}

	return &Info{
		URL:          repoURL,
		Revision:     currentRev,
		TargetDir:    targetDir,
		CheckedOutAt: time.Now(),
	}, nil
}

func (s *SVNService) doCheckout(ctx context.Context, repoURL, targetDir, revision string, creds credentials.Provider) error {
	args := []string{"checkout", repoURL, targetDir, "--non-interactive", "--no-auth-cache", "--trust-server-cert"}

	if revision != "" && revision != "HEAD" {
		args = append(args, "--revision", revision)
	}

	args, pass := s.appendCredArgs(args, creds)

	opts := models.ExecOptions{
		Command: "svn",
		Args:    args,
	}

	res, err := s.runner.Run(ctx, opts)
	if err != nil || !res.Successful {
		errMsg := res.ErrorMessage
		if errMsg == "" && err != nil {
			errMsg = err.Error()
		}
		cleanMsg := s.maskPassword(errMsg, pass)
		return fmt.Errorf("svn checkout failed: %s", cleanMsg)
	}
	return nil
}

func (s *SVNService) doUpdate(ctx context.Context, targetDir, revision string, creds credentials.Provider) error {
	args := []string{"update", targetDir, "--non-interactive", "--no-auth-cache", "--trust-server-cert"}

	if revision != "" && revision != "HEAD" {
		args = append(args, "--revision", revision)
	}

	args, pass := s.appendCredArgs(args, creds)

	opts := models.ExecOptions{
		Command: "svn",
		Args:    args,
		Dir:     targetDir,
	}

	res, err := s.runner.Run(ctx, opts)
	if err != nil || !res.Successful {
		errMsg := res.ErrorMessage
		if errMsg == "" && err != nil {
			errMsg = err.Error()
		}
		cleanMsg := s.maskPassword(errMsg, pass)
		return fmt.Errorf("svn update failed: %s", cleanMsg)
	}
	return nil
}

// GetRevision fetches revision info from svn info.
func (s *SVNService) GetRevision(ctx context.Context, targetDir string) (string, error) {
	opts := models.ExecOptions{
		Command: "svn",
		Args:    []string{"info", "--show-item", "revision", targetDir},
		Dir:     targetDir,
	}

	res, err := s.runner.Run(ctx, opts)
	if err != nil || !res.Successful {
		return "UNKNOWN", nil
	}

	rev := strings.TrimSpace(res.StdOut)
	if rev == "" {
		return "HEAD", nil
	}
	return rev, nil
}

func (s *SVNService) appendCredArgs(args []string, creds credentials.Provider) ([]string, string) {
	if creds == nil {
		return args, ""
	}
	user := creds.Username()
	pass, _ := creds.Password()

	if user != "" {
		args = append(args, "--username", user)
	}
	if pass != "" {
		args = append(args, "--password", pass)
	}
	return args, pass
}

func (s *SVNService) maskPassword(msg, password string) string {
	if password != "" && strings.Contains(msg, password) {
		return strings.ReplaceAll(msg, password, "********")
	}
	return msg
}
