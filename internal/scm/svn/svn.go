package svn

import (
	"context"

	"github.com/anandhuremanan/cicd-engine/internal/credentials"
	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/process"
	"github.com/anandhuremanan/cicd-engine/internal/repository"
	"github.com/anandhuremanan/cicd-engine/internal/scm"
)

// Provider implements scm.Provider for SVN repositories.
type Provider struct {
	service *repository.SVNService
}

// NewProvider constructs an SVN Provider wrapping repository.SVNService.
func NewProvider(runner process.Runner, log logger.Logger) scm.Provider {
	return &Provider{
		service: repository.NewSVNService(runner, log),
	}
}

// SetProgress registers a callback that receives the number of files downloaded during a checkout.
func (p *Provider) SetProgress(fn func(files int)) {
	p.service.SetProgress(fn)
}

func (p *Provider) Name() string {
	return "SVN"
}

func (p *Provider) CheckoutOrUpdate(ctx context.Context, repoURL, targetDir, revision string, creds credentials.Provider) (*repository.Info, error) {
	return p.service.CheckoutOrUpdate(ctx, repoURL, targetDir, revision, creds)
}

// CheckoutFolders downloads only the named folders of the repository, plus its top-level files.
func (p *Provider) CheckoutFolders(ctx context.Context, repoURL, targetDir, revision string, folders []string, creds credentials.Provider) (*repository.Info, error) {
	return p.service.CheckoutFolders(ctx, repoURL, targetDir, revision, folders, creds)
}

// DeepenAll downloads everything a CheckoutFolders working copy left out.
func (p *Provider) DeepenAll(ctx context.Context, targetDir, revision string, creds credentials.Provider) error {
	return p.service.DeepenAll(ctx, targetDir, revision, creds)
}

func (p *Provider) GetRevision(ctx context.Context, targetDir string) (string, error) {
	return p.service.GetRevision(ctx, targetDir)
}
