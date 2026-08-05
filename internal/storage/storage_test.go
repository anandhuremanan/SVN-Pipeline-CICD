package storage_test

import (
	"testing"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/storage/builds"
	"github.com/anandhuremanan/cicd-engine/internal/storage/logs"
)

func TestBuildStore(t *testing.T) {
	tempDir := t.TempDir()
	store, err := builds.NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create build store: %v", err)
	}

	job := models.Job{
		ID:         "job-100",
		Repository: "https://svn.example.com/repo",
		Revision:   "100",
		Status:     models.JobStatusSuccess,
		StartedAt:  time.Now().Add(-5 * time.Minute),
		FinishedAt: time.Now(),
		Duration:   5 * time.Minute,
	}

	if err := store.Save(job); err != nil {
		t.Fatalf("failed to save job: %v", err)
	}

	retrieved, err := store.Get("job-100")
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}

	if retrieved.ID != "job-100" || retrieved.Status != models.JobStatusSuccess {
		t.Errorf("unexpected retrieved job: %+v", retrieved)
	}

	list, err := store.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 job in list, got %d (err: %v)", len(list), err)
	}
}

func TestLogStore(t *testing.T) {
	tempDir := t.TempDir()
	logStore, err := logs.NewFileLogStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	entry1 := logs.LogEntry{Project: "client", Step: "build", Level: "INFO", Message: "compiling"}
	entry2 := logs.LogEntry{Project: "client", Step: "build", Level: "INFO", Message: "done"}

	if err := logStore.Append("job-200", entry1); err != nil {
		t.Fatalf("failed to append log1: %v", err)
	}
	if err := logStore.Append("job-200", entry2); err != nil {
		t.Fatalf("failed to append log2: %v", err)
	}

	readEntries, err := logStore.Read("job-200")
	if err != nil {
		t.Fatalf("failed to read logs: %v", err)
	}

	if len(readEntries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(readEntries))
	}
}
