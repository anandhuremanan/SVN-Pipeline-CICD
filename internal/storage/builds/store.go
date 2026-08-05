package builds

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// BuildStore defines operations for persisting and querying completed build jobs.
type BuildStore interface {
	// Save persists a job record to disk.
	Save(job models.Job) error
	// Get retrieves a job by ID.
	Get(id string) (*models.Job, error)
	// List returns all stored jobs sorted reverse-chronologically.
	List() ([]models.Job, error)
}

// FileStore implements BuildStore using JSON files.
type FileStore struct {
	mu      sync.RWMutex
	baseDir string
}

// NewFileStore creates a FileStore instance in baseDir.
func NewFileStore(baseDir string) (*FileStore, error) {
	if baseDir == "" {
		baseDir = filepath.Join(".", ".storage", "builds")
	}

	absDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("build store: invalid path %q: %w", baseDir, err)
	}

	if err := os.MkdirAll(absDir, 0755); err != nil {
		return nil, fmt.Errorf("build store: failed to create dir %q: %w", absDir, err)
	}

	return &FileStore{baseDir: absDir}, nil
}

// Save marshals and writes job record to JSON file.
func (s *FileStore) Save(job models.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job.ID == "" {
		return fmt.Errorf("build store: job ID cannot be empty")
	}

	filePath := filepath.Join(s.baseDir, job.ID+".json")
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return fmt.Errorf("build store: failed to marshal job %s: %w", job.ID, err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("build store: failed to write file %s: %w", filePath, err)
	}

	return nil
}

// Get reads and parses a job JSON file.
func (s *FileStore) Get(id string) (*models.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := filepath.Join(s.baseDir, id+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("build store: failed to read job %s: %w", id, err)
	}

	var job models.Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("build store: failed to unmarshal job %s: %w", id, err)
	}

	return &job, nil
}

// List scans baseDir for job files and returns them sorted by start time.
func (s *FileStore) List() ([]models.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("build store: failed to read dir: %w", err)
	}

	var jobs []models.Job
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		filePath := filepath.Join(s.baseDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		var job models.Job
		if err := json.Unmarshal(data, &job); err == nil {
			jobs = append(jobs, job)
		}
	}

	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].StartedAt.After(jobs[j].StartedAt)
	})

	return jobs, nil
}
