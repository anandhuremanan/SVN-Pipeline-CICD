package service

import (
	"sync"

	"github.com/anandhuremanan/cicd-engine/internal/storage/logs"
)

// LogBroadcaster manages SSE subscribers and fan-outs real-time log entries to connected clients.
type LogBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan logs.LogEntry]struct{}
}

// NewLogBroadcaster creates a new LogBroadcaster.
func NewLogBroadcaster() *LogBroadcaster {
	return &LogBroadcaster{
		subscribers: make(map[string]map[chan logs.LogEntry]struct{}),
	}
}

// Subscribe returns a channel that receives live LogEntry events for jobID, and an unsubscribe function.
func (b *LogBroadcaster) Subscribe(jobID string) (<-chan logs.LogEntry, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan logs.LogEntry, 100)

	if _, exists := b.subscribers[jobID]; !exists {
		b.subscribers[jobID] = make(map[chan logs.LogEntry]struct{})
	}
	b.subscribers[jobID][ch] = struct{}{}

	unsubscribe := func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		if subs, exists := b.subscribers[jobID]; exists {
			if _, ok := subs[ch]; ok {
				delete(subs, ch)
				close(ch)
			}
			if len(subs) == 0 {
				delete(b.subscribers, jobID)
			}
		}
	}

	return ch, unsubscribe
}

// Publish broadcasts a LogEntry to all active subscribers for jobID.
func (b *LogBroadcaster) Publish(jobID string, entry logs.LogEntry) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subs, exists := b.subscribers[jobID]
	if !exists {
		return
	}

	for ch := range subs {
		select {
		case ch <- entry:
		default:
			// Non-blocking write: drop log if subscriber channel buffer is full to prevent blocking build execution
		}
	}
}
