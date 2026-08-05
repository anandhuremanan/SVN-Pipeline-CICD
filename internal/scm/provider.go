package scm

import (
	"context"

	"github.com/anandhuremanan/cicd-engine/internal/credentials"
	"github.com/anandhuremanan/cicd-engine/internal/repository"
)

// Provider defines abstract operations for Source Control Management (SCM) systems.
type Provider interface {
	// Name returns the SCM system name (e.g. "SVN", "Git").
	Name() string
	// CheckoutOrUpdate checks out or updates a working copy directory.
	CheckoutOrUpdate(ctx context.Context, repoURL, targetDir, revision string, creds credentials.Provider) (*repository.Info, error)
	// GetRevision queries the current working copy revision.
	GetRevision(ctx context.Context, targetDir string) (string, error)
}
