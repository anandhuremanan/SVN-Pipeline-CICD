package pipeline

import (
	"context"
	"sync"
)

// PipelineContext holds runtime environment, configuration, credentials, repository parameters,
// and workspace information needed across pipeline stage executions.
// It explicitly excludes stage output results (such as detected projects or build results),
// treating stages as pure data transformations.
type PipelineContext struct {
	mu sync.RWMutex

	// Ctx carries cancellation signals and execution deadlines.
	Ctx context.Context
	// RepoURL is the target repository URL (e.g., SVN URL).
	RepoURL string
	// Revision is the target revision (defaults to "HEAD").
	Revision string
	// Username for repository authentication.
	Username string
	// password is kept unexported for security to prevent log or reflection leaks.
	password string
	// WorkspaceDir is the root path of the isolated build workspace.
	WorkspaceDir string
	// KeepWorkspace specifies whether to retain workspace after pipeline completion.
	KeepWorkspace bool
}

// NewPipelineContext constructs a new PipelineContext with sensible defaults.
func NewPipelineContext(ctx context.Context, repoURL, username, password string) *PipelineContext {
	if ctx == nil {
		ctx = context.Background()
	}
	return &PipelineContext{
		Ctx:      ctx,
		RepoURL:  repoURL,
		Username: username,
		password: password,
		Revision: "HEAD",
	}
}

// SetPassword updates the authentication password in a thread-safe manner.
func (pc *PipelineContext) SetPassword(pass string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.password = pass
}

// Password returns the authentication password in a thread-safe manner.
func (pc *PipelineContext) Password() string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.password
}
