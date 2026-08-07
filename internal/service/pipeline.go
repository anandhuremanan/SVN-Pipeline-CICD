package service

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/cli"
	"github.com/anandhuremanan/cicd-engine/internal/config"
	"github.com/anandhuremanan/cicd-engine/internal/doctor"
	"github.com/anandhuremanan/cicd-engine/internal/engine"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/storage/builds"
	"github.com/anandhuremanan/cicd-engine/internal/storage/logs"
)

// PipelineService provides unified business logic for pipeline execution, history, and status.
type PipelineService interface {
	Run(ctx context.Context, request cli.RunRequest) error
	ExecuteRun(ctx context.Context, request cli.RunRequest) (string, error)
	RunAsync(ctx context.Context, request cli.RunRequest) (string, error)
	History() ([]models.Job, error)
	GetHistory() ([]models.Job, error)
	Status(jobID string) (*models.Job, error)
	GetStatus() ([]models.Job, error)
	Logs(jobID string) ([]logs.LogEntry, error)
	RunningJobs() ([]models.Job, error)
	Subscribe(jobID string) (<-chan logs.LogEntry, func())
	PublishLog(jobID string, entry logs.LogEntry)
	Doctor(ctx context.Context) (*doctor.Result, error)
}

// BroadcastingLogStore decorates LogStore to publish entries to LogBroadcaster.
type BroadcastingLogStore struct {
	inner       logs.LogStore
	broadcaster *LogBroadcaster
}

// NewBroadcastingLogStore constructs a BroadcastingLogStore wrapper.
func NewBroadcastingLogStore(inner logs.LogStore, broadcaster *LogBroadcaster) *BroadcastingLogStore {
	return &BroadcastingLogStore{
		inner:       inner,
		broadcaster: broadcaster,
	}
}

func (b *BroadcastingLogStore) Append(jobID string, entry logs.LogEntry) error {
	err := b.inner.Append(jobID, entry)
	if b.broadcaster != nil {
		b.broadcaster.Publish(jobID, entry)
	}
	return err
}

func (b *BroadcastingLogStore) Read(jobID string) ([]logs.LogEntry, error) {
	return b.inner.Read(jobID)
}

// DefaultPipelineService implements PipelineService using Engine, BuildStore, and LogStore.
type DefaultPipelineService struct {
	engine      *engine.Engine
	buildStore  builds.BuildStore
	logStore    logs.LogStore
	broadcaster *LogBroadcaster
	out         io.Writer
}

// NewDefaultPipelineService creates a DefaultPipelineService instance.
func NewDefaultPipelineService(
	eng *engine.Engine,
	bStore builds.BuildStore,
	lStore logs.LogStore,
	broadcaster *LogBroadcaster,
	out io.Writer,
) *DefaultPipelineService {
	if bStore == nil {
		bStore, _ = builds.NewFileStore("")
	}
	if broadcaster == nil {
		broadcaster = NewLogBroadcaster()
	}
	if lStore == nil {
		baseLStore, _ := logs.NewFileLogStore("")
		lStore = NewBroadcastingLogStore(baseLStore, broadcaster)
	} else {
		// Wrap provided logStore if not already wrapped
		lStore = NewBroadcastingLogStore(lStore, broadcaster)
	}
	if eng == nil {
		eng = engine.NewEngine(out, nil, nil, nil, nil, nil, nil, nil, bStore, lStore, nil)
	}

	return &DefaultPipelineService{
		engine:      eng,
		buildStore:  bStore,
		logStore:    lStore,
		broadcaster: broadcaster,
		out:         out,
	}
}

// Run executes a build job synchronously (satisfies cli.Engine).
func (s *DefaultPipelineService) Run(ctx context.Context, request cli.RunRequest) error {
	return s.engine.Run(ctx, request)
}

// ExecuteRun executes a build job synchronously and returns the job ID.
func (s *DefaultPipelineService) ExecuteRun(ctx context.Context, request cli.RunRequest) (string, error) {
	err := s.engine.Run(ctx, request)
	if err != nil {
		return "", err
	}
	history, hErr := s.History()
	if hErr == nil && len(history) > 0 {
		return history[0].ID, nil
	}
	return "", nil
}

// RunAsync launches a build job asynchronously in a background goroutine and returns the target job ID.
func (s *DefaultPipelineService) RunAsync(ctx context.Context, request cli.RunRequest) (string, error) {
	errChan := make(chan error, 1)
	bgCtx := context.Background()

	go func() {
		err := s.engine.Run(bgCtx, request)
		if err != nil {
			errChan <- err
		}
	}()

	// Wait briefly to allow engine to generate job ID or return an immediate error
	select {
	case err := <-errChan:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	case <-func() <-chan struct{} {
		ch := make(chan struct{})
		go func() {
			for i := 0; i < 50; i++ {
				time.Sleep(10 * time.Millisecond)
				if s.buildStore != nil {
					list, err := s.buildStore.List()
					if err == nil && len(list) > 0 {
						ch <- struct{}{}
						return
					}
				}
				history, err := s.History()
				if err == nil && len(history) > 0 {
					ch <- struct{}{}
					return
				}
				running, rErr := s.RunningJobs()
				if rErr == nil && len(running) > 0 {
					ch <- struct{}{}
					return
				}
			}
			ch <- struct{}{}
		}()
		return ch
	}():
		history, hErr := s.History()
		if hErr == nil && len(history) > 0 {
			return history[0].ID, nil
		}
		running, rErr := s.RunningJobs()
		if rErr == nil && len(running) > 0 {
			return running[0].ID, nil
		}
	}

	return "", nil
}

// History retrieves all completed build jobs.
func (s *DefaultPipelineService) History() ([]models.Job, error) {
	return s.engine.GetHistory()
}

// GetHistory satisfies the cli.Engine interface for history command.
func (s *DefaultPipelineService) GetHistory() ([]models.Job, error) {
	return s.History()
}

// Status retrieves a single job details by ID (from buildStore or running queue).
func (s *DefaultPipelineService) Status(jobID string) (*models.Job, error) {
	// 1. Check buildStore first (authoritative persistent storage on disk)
	if s.buildStore != nil {
		j, err := s.buildStore.Get(jobID)
		if err == nil && j != nil && j.ID == jobID {
			return j, nil
		}
	}

	// 2. Check finished history
	history, err := s.engine.GetHistory()
	if err == nil {
		for _, j := range history {
			if j.ID == jobID {
				return &j, nil
			}
		}
	}

	// 3. Fallback to running queue
	running, err := s.engine.GetStatus()
	if err == nil {
		for _, j := range running {
			if j.ID == jobID {
				return &j, nil
			}
		}
	}

	return nil, fmt.Errorf("job %q not found", jobID)
}

// GetStatus satisfies the cli.Engine interface for status command.
func (s *DefaultPipelineService) GetStatus() ([]models.Job, error) {
	return s.RunningJobs()
}

// Logs returns stored log entries for a job ID.
func (s *DefaultPipelineService) Logs(jobID string) ([]logs.LogEntry, error) {
	return s.logStore.Read(jobID)
}

// RunningJobs returns currently active or queued build jobs.
func (s *DefaultPipelineService) RunningJobs() ([]models.Job, error) {
	return s.engine.GetStatus()
}

// Subscribe returns an SSE log stream channel and unsubscribe callback.
func (s *DefaultPipelineService) Subscribe(jobID string) (<-chan logs.LogEntry, func()) {
	return s.broadcaster.Subscribe(jobID)
}

// PublishLog appends and broadcasts a custom log entry.
func (s *DefaultPipelineService) PublishLog(jobID string, entry logs.LogEntry) {
	_ = s.logStore.Append(jobID, entry)
}

// Doctor runs system tool health diagnostics.
func (s *DefaultPipelineService) Doctor(ctx context.Context) (*doctor.Result, error) {
	cfg, _ := config.Load("")
	doc := doctor.NewDoctor(nil, nil)
	out := s.out
	if out == nil {
		out = io.Discard
	}
	return doc.RunDiagnostics(ctx, out, cfg)
}
