package pipeline_test

import (
	"context"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/pipeline"
)

func TestPipelineContext(t *testing.T) {
	ctx := context.Background()
	pCtx := pipeline.NewPipelineContext(ctx, "https://svn.example.com/repo", "devuser", "secretpass")

	if pCtx.RepoURL != "https://svn.example.com/repo" {
		t.Errorf("expected RepoURL %q, got %q", "https://svn.example.com/repo", pCtx.RepoURL)
	}

	if pCtx.Password() != "secretpass" {
		t.Errorf("expected password %q, got %q", "secretpass", pCtx.Password())
	}

	pCtx.SetPassword("newsecret")
	if pCtx.Password() != "newsecret" {
		t.Errorf("expected updated password %q, got %q", "newsecret", pCtx.Password())
	}

	if pCtx.Revision != "HEAD" {
		t.Errorf("expected default Revision 'HEAD', got %q", pCtx.Revision)
	}
}
