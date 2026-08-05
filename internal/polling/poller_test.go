package polling_test

import (
	"context"
	"testing"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/credentials"
	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/polling"
	"github.com/anandhuremanan/cicd-engine/internal/queue"
	"github.com/anandhuremanan/cicd-engine/internal/repository"
)

type mockSCMProvider struct {
	revision string
}

func (m *mockSCMProvider) Name() string {
	return "MockSCM"
}

func (m *mockSCMProvider) CheckoutOrUpdate(ctx context.Context, repoURL, targetDir, revision string, creds credentials.Provider) (*repository.Info, error) {
	return &repository.Info{URL: repoURL, Revision: m.revision, TargetDir: targetDir}, nil
}

func (m *mockSCMProvider) GetRevision(ctx context.Context, targetDir string) (string, error) {
	return m.revision, nil
}

func TestPollerSubmitsJobOnRevisionChange(t *testing.T) {
	q := queue.NewMemoryQueue(10)
	scmMock := &mockSCMProvider{revision: "r100"}

	poller := polling.NewPoller(scmMock, q, "https://svn.example.com/repo", nil, 100*time.Millisecond, logger.NopLogger())

	// First poll should detect r100 and submit job
	if err := poller.PollOnce(context.Background()); err != nil {
		t.Fatalf("unexpected PollOnce error: %v", err)
	}

	jobs := q.List()
	if len(jobs) != 1 || jobs[0].Revision != "r100" {
		t.Fatalf("expected 1 job with revision r100, got %+v", jobs)
	}

	// Second poll with same revision should NOT submit another job
	if err := poller.PollOnce(context.Background()); err != nil {
		t.Fatalf("unexpected PollOnce error: %v", err)
	}
	if len(q.List()) != 1 {
		t.Errorf("expected queue length to remain 1 when revision does not change")
	}

	// Update revision to r101 and poll again
	scmMock.revision = "r101"
	if err := poller.PollOnce(context.Background()); err != nil {
		t.Fatalf("unexpected PollOnce error: %v", err)
	}
	if len(q.List()) != 2 {
		t.Errorf("expected queue length to be 2 after revision update to r101")
	}
}
