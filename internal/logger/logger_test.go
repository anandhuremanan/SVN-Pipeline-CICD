package logger_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/logger"
)

func TestNewLoggerLevels(t *testing.T) {
	var buf bytes.Buffer
	log := logger.NewLogger(&buf, "DEBUG")

	log.Debug("test debug")
	if !strings.Contains(buf.String(), "test debug") {
		t.Errorf("expected debug message to be logged, got: %s", buf.String())
	}
}

func TestLoggerFiltering(t *testing.T) {
	var buf bytes.Buffer
	log := logger.NewLogger(&buf, "WARN")

	log.Info("test info")
	if buf.Len() > 0 {
		t.Errorf("expected info message to be filtered out at WARN level, got: %s", buf.String())
	}

	log.Warn("test warn")
	if !strings.Contains(buf.String(), "test warn") {
		t.Errorf("expected warn message to be logged, got: %s", buf.String())
	}
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer
	log := logger.NewLogger(&buf, "INFO")

	subLog := log.With("module", "checkout")
	subLog.Info("starting checkout")

	output := buf.String()
	if !strings.Contains(output, "module=checkout") || !strings.Contains(output, "starting checkout") {
		t.Errorf("expected contextual fields in log output, got: %s", output)
	}
}

func TestMaskSensitive(t *testing.T) {
	sensitiveURL := "https://myuser:secretpassword123@svn.example.com/repo/trunk"
	masked := logger.MaskSensitive(sensitiveURL)

	if strings.Contains(masked, "secretpassword123") {
		t.Errorf("expected password to be masked, got: %s", masked)
	}

	if !strings.Contains(masked, "https://****:****@svn.example.com/repo/trunk") {
		t.Errorf("expected masked URL format, got: %s", masked)
	}
}
