package service_test

import (
	"testing"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/service"
	"github.com/anandhuremanan/cicd-engine/internal/storage/logs"
)

func TestLogBroadcaster_PubSub(t *testing.T) {
	broadcaster := service.NewLogBroadcaster()

	ch, unsubscribe := broadcaster.Subscribe("job-123")
	defer unsubscribe()

	entry := logs.LogEntry{
		Time:    time.Now(),
		JobID:   "job-123",
		Project: "Client",
		Step:    "Build",
		Level:   "INFO",
		Message: "Compilation succeeded",
	}

	broadcaster.Publish("job-123", entry)

	select {
	case received := <-ch:
		if received.Message != entry.Message {
			t.Errorf("expected message %q, got %q", entry.Message, received.Message)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for log entry event")
	}
}

func TestPipelineService_BroadcastingLogStore(t *testing.T) {
	svc := service.NewDefaultPipelineService(nil, nil, nil, nil, nil)

	ch, unsubscribe := svc.Subscribe("job-456")
	defer unsubscribe()

	entry := logs.LogEntry{
		JobID:   "job-456",
		Message: "Test log message",
	}

	svc.PublishLog("job-456", entry)

	select {
	case received := <-ch:
		if received.Message != "Test log message" {
			t.Errorf("expected %q, got %q", "Test log message", received.Message)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for published log entry")
	}
}
