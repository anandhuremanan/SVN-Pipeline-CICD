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
	service repository.Service
}

// NewProvider constructs an SVN Provider wrapping repository.SVNService.
func NewProvider(runner process.Runner, log logger.Logger) scm.Provider {
	return &Provider{
		service: repository.NewSVNService(runner, log),
	}
}

func (p *Provider) Name() string {
	return "SVN"
}

func (p *Provider) CheckoutOrUpdate(ctx context.Context, repoURL, targetDir, revision string, creds credentials.Provider) (*repository.Info, error) {
	return p.service.CheckoutOrUpdate(ctx, repoURL, targetDir, revision, creds)
}

func (p *Provider) GetRevision(ctx context.Context, targetDir string) (string, error) {
	return p.service.GetRevision(ctx, targetDir)
}
