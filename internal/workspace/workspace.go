package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/logger"
)

// Workspace represents an isolated build workspace directory.
type Workspace struct {
	// ID is the unique identifier for the workspace.
	ID string `json:"id"`
	// Path is the absolute directory path of the workspace on disk.
	Path string `json:"path"`
	// KeepWorkspace determines if the directory should be preserved on cleanup.
	KeepWorkspace bool `json:"keep_workspace"`
}

// Manager defines lifecycle operations for isolated build workspaces.
type Manager interface {
	// CreateWorkspace allocates and initializes a unique temporary workspace directory.
	CreateWorkspace(prefix string) (*Workspace, error)
	// Cleanup removes the workspace directory unless KeepWorkspace is enabled.
	Cleanup(ws *Workspace) error
}

// LocalManager implements Manager by managing workspace folders on the local file system.
type LocalManager struct {
	rootDir       string
	keepWorkspace bool
	logger        logger.Logger
}

// NewManager constructs a LocalManager workspace manager instance.
func NewManager(rootDir string, keepWorkspace bool, log logger.Logger) (*LocalManager, error) {
	if rootDir == "" {
		rootDir = filepath.Join(".", ".workspaces")
	}
	if log == nil {
		log = logger.NopLogger()
	}

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("workspace manager: invalid root path %q: %w", rootDir, err)
	}

	if err := os.MkdirAll(absRoot, 0755); err != nil {
		return nil, fmt.Errorf("workspace manager: failed to create root dir %q: %w", absRoot, err)
	}

	return &LocalManager{
		rootDir:       absRoot,
		keepWorkspace: keepWorkspace,
		logger:        log,
	}, nil
}

// CreateWorkspace allocates an isolated directory under rootDir for a build.
func (m *LocalManager) CreateWorkspace(prefix string) (*Workspace, error) {
	if prefix == "" {
		prefix = "build"
	}

	timestamp := time.Now().Format("20060102-150405")
	pattern := fmt.Sprintf("%s-%s-*", prefix, timestamp)

	wsPath, err := os.MkdirTemp(m.rootDir, pattern)
	if err != nil {
		return nil, fmt.Errorf("workspace manager: failed to create workspace directory: %w", err)
	}

	wsID := filepath.Base(wsPath)
	m.logger.Info("created isolated workspace", "id", wsID, "path", wsPath)

	return &Workspace{
		ID:            wsID,
		Path:          wsPath,
		KeepWorkspace: m.keepWorkspace,
	}, nil
}

// Cleanup removes the workspace directory from disk unless KeepWorkspace is enabled.
func (m *LocalManager) Cleanup(ws *Workspace) error {
	if ws == nil || ws.Path == "" {
		return nil
	}

	if ws.KeepWorkspace || m.keepWorkspace {
		m.logger.Info("preserving workspace directory as requested", "id", ws.ID, "path", ws.Path)
		return nil
	}

	m.logger.Info("cleaning up workspace directory", "id", ws.ID, "path", ws.Path)
	if err := os.RemoveAll(ws.Path); err != nil {
		return fmt.Errorf("workspace manager: failed to cleanup workspace at %q: %w", ws.Path, err)
	}

	return nil
}
