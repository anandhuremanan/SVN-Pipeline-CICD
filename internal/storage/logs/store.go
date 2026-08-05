package logs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogEntry encapsulates a structured build log event record.
type LogEntry struct {
	Time    time.Time `json:"time"`
	JobID   string    `json:"job_id"`
	Project string    `json:"project"`
	Step    string    `json:"step"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// LogStore defines operations for appending and reading build job log files.
type LogStore interface {
	// Append writes a structured log line to the job's log file.
	Append(jobID string, entry LogEntry) error
	// Read reads all log entries recorded for a job.
	Read(jobID string) ([]LogEntry, error)
}

// FileLogStore implements LogStore using JSON Lines files.
type FileLogStore struct {
	mu      sync.Mutex
	baseDir string
}

// NewFileLogStore creates a FileLogStore in baseDir.
func NewFileLogStore(baseDir string) (*FileLogStore, error) {
	if baseDir == "" {
		baseDir = filepath.Join(".", ".storage", "logs")
	}

	absDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("log store: invalid path %q: %w", baseDir, err)
	}

	if err := os.MkdirAll(absDir, 0755); err != nil {
		return nil, fmt.Errorf("log store: failed to create dir %q: %w", absDir, err)
	}

	return &FileLogStore{baseDir: absDir}, nil
}

// Append serializes and appends a LogEntry line to jobID.log.
func (s *FileLogStore) Append(jobID string, entry LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if jobID == "" {
		return fmt.Errorf("log store: job ID cannot be empty")
	}

	entry.JobID = jobID
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}

	filePath := filepath.Join(s.baseDir, jobID+".log")
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("log store: failed to open log file %s: %w", filePath, err)
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("log store: failed to marshal log entry: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("log store: failed to append log line: %w", err)
	}

	return nil
}

// Read reads and unmarshals all log lines from jobID.log.
func (s *FileLogStore) Read(jobID string) ([]LogEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.baseDir, jobID+".log")
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []LogEntry{}, nil
		}
		return nil, fmt.Errorf("log store: failed to open log file %s: %w", filePath, err)
	}
	defer f.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry LogEntry
		if err := json.Unmarshal(line, &entry); err == nil {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}
