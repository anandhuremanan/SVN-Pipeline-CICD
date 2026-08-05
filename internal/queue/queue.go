package queue

import (
	"context"
	"fmt"
	"sync"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// Queue abstracts job scheduling and queueing operations.
type Queue interface {
	// Submit pushes a new Job into the queue.
	Submit(job models.Job) error
	// Next retrieves the next pending Job, blocking until available or context is cancelled.
	Next(ctx context.Context) (*models.Job, error)
	// List returns all queued or processed jobs in order.
	List() []models.Job
}

// MemoryQueue is a thread-safe in-memory queue implementation.
type MemoryQueue struct {
	mu    sync.RWMutex
	ch    chan models.Job
	jobs  map[string]models.Job
	order []string
}

// NewMemoryQueue constructs a MemoryQueue instance with channel capacity.
func NewMemoryQueue(capacity int) *MemoryQueue {
	if capacity < 1 {
		capacity = 100
	}
	return &MemoryQueue{
		ch:    make(chan models.Job, capacity),
		jobs:  make(map[string]models.Job),
		order: make([]string, 0),
	}
}

// Submit enqueues a Job.
func (q *MemoryQueue) Submit(job models.Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if job.ID == "" {
		return fmt.Errorf("queue: job ID cannot be empty")
	}

	job.Status = models.JobStatusPending
	q.jobs[job.ID] = job
	q.order = append(q.order, job.ID)

	select {
	case q.ch <- job:
		return nil
	default:
		return fmt.Errorf("queue: submit failed, queue is full")
	}
}

// Next pops the next Job from the queue.
func (q *MemoryQueue) Next(ctx context.Context) (*models.Job, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case job, ok := <-q.ch:
		if !ok {
			return nil, fmt.Errorf("queue: queue channel closed")
		}
		q.mu.Lock()
		job.Status = models.JobStatusRunning
		q.jobs[job.ID] = job
		q.mu.Unlock()
		return &job, nil
	}
}

// List returns a snapshot of all submitted jobs.
func (q *MemoryQueue) List() []models.Job {
	q.mu.RLock()
	defer q.mu.RUnlock()

	list := make([]models.Job, 0, len(q.order))
	for _, id := range q.order {
		if job, ok := q.jobs[id]; ok {
			list = append(list, job)
		}
	}
	return list
}
