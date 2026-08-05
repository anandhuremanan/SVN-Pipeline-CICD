package polling

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/credentials"
	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/queue"
	"github.com/anandhuremanan/cicd-engine/internal/scm"
)

// Poller periodically queries a repository for revision updates and queues build Jobs.
type Poller struct {
	mu           sync.Mutex
	scmProvider  scm.Provider
	queue        queue.Queue
	logger       logger.Logger
	repoURL      string
	creds        credentials.Provider
	interval     time.Duration
	lastRevision string
	running      bool
	stopCh       chan struct{}
}

// NewPoller constructs a Poller instance.
func NewPoller(scmProvider scm.Provider, q queue.Queue, repoURL string, creds credentials.Provider, interval time.Duration, log logger.Logger) *Poller {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	if log == nil {
		log = logger.NopLogger()
	}
	return &Poller{
		scmProvider: scmProvider,
		queue:       q,
		repoURL:     repoURL,
		creds:       creds,
		interval:    interval,
		logger:      log,
		stopCh:      make(chan struct{}),
	}
}

// PollOnce checks the repository for revision changes and submits a Job if changed.
func (p *Poller) PollOnce(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.logger.Info("polling repository for revision changes", "url", p.repoURL)
	rev, err := p.scmProvider.GetRevision(ctx, p.repoURL)
	if err != nil {
		p.logger.Warn("failed to poll repository revision", "url", p.repoURL, "err", err)
		return fmt.Errorf("poller: failed to get revision: %w", err)
	}

	if rev != "" && rev != p.lastRevision {
		p.logger.Info("detected new repository revision", "url", p.repoURL, "old", p.lastRevision, "new", rev)
		p.lastRevision = rev

		jobID := fmt.Sprintf("job-%s-%d", rev, time.Now().UnixNano()%100000)
		job := models.Job{
			ID:         jobID,
			Repository: p.repoURL,
			Revision:   rev,
			Status:     models.JobStatusPending,
			StartedAt:  time.Now(),
		}

		if err := p.queue.Submit(job); err != nil {
			p.logger.Error("failed to submit polled job to queue", "job_id", jobID, "err", err)
			return err
		}
	} else {
		p.logger.Debug("no revision change detected during polling", "url", p.repoURL, "revision", rev)
	}

	return nil
}

// Start launches the background polling ticker loop.
func (p *Poller) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("poller is already running")
	}
	p.running = true
	p.mu.Unlock()

	p.logger.Info("starting continuous repository poller", "url", p.repoURL, "interval", p.interval)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	_ = p.PollOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.stopCh:
			return nil
		case <-ticker.C:
			_ = p.PollOnce(ctx)
		}
	}
}

// Stop signals the polling loop to terminate.
func (p *Poller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		close(p.stopCh)
		p.running = false
	}
}
