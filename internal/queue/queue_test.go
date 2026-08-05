package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/queue"
)

func TestMemoryQueueSubmitAndNext(t *testing.T) {
	q := queue.NewMemoryQueue(10)

	job1 := models.Job{ID: "job-1", Repository: "https://svn.example.com/repo1"}
	job2 := models.Job{ID: "job-2", Repository: "https://svn.example.com/repo2"}

	if err := q.Submit(job1); err != nil {
		t.Fatalf("failed to submit job1: %v", err)
	}
	if err := q.Submit(job2); err != nil {
		t.Fatalf("failed to submit job2: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	popped1, err := q.Next(ctx)
	if err != nil || popped1.ID != "job-1" {
		t.Fatalf("expected job-1, got %+v (err: %v)", popped1, err)
	}
	if popped1.Status != models.JobStatusRunning {
		t.Errorf("expected status RUNNING, got %s", popped1.Status)
	}

	popped2, err := q.Next(ctx)
	if err != nil || popped2.ID != "job-2" {
		t.Fatalf("expected job-2, got %+v (err: %v)", popped2, err)
	}

	jobs := q.List()
	if len(jobs) != 2 {
		t.Errorf("expected 2 listed jobs, got %d", len(jobs))
	}
}
