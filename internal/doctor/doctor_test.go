package doctor_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/config"
	"github.com/anandhuremanan/cicd-engine/internal/doctor"
	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/process"
)

func TestDoctorRunDiagnosticsSuccess(t *testing.T) {
	var buf bytes.Buffer

	mockRunner := process.NewMockRunner()
	mockRunner.RunFunc = func(ctx context.Context, opts models.ExecOptions, handlers ...process.StreamHandler) (models.ProcessResult, error) {
		return models.NewProcessResult(opts.Command, 0, "1.0.0", "", 0, nil), nil
	}

	doc := doctor.NewDoctor(mockRunner, logger.NopLogger())
	res, err := doc.RunDiagnostics(context.Background(), &buf, config.DefaultConfig())

	if err != nil {
		t.Fatalf("unexpected doctor error: %v", err)
	}

	if !res.AllOK {
		t.Errorf("expected AllOK true, got false")
	}

	outStr := buf.String()

	expectedItems := []string{
		"Environment",
		"✓ SVN",
		"✓ Node",
		"✓ NPM",
		"✓ Dotnet",
		"Workspace",
		"✓ Writable",
		"Storage",
		"✓ Writable",
		"Credentials",
		"✓ Available",
		"Everything OK",
	}

	for _, expected := range expectedItems {
		if !strings.Contains(outStr, expected) {
			t.Errorf("expected doctor output to contain %q, got:\n%s", expected, outStr)
		}
	}
}

func TestDoctorRunDiagnosticsMissingTool(t *testing.T) {
	var buf bytes.Buffer

	mockRunner := process.NewMockRunner()
	mockRunner.RunFunc = func(ctx context.Context, opts models.ExecOptions, handlers ...process.StreamHandler) (models.ProcessResult, error) {
		if opts.Command == "dotnet" {
			return models.NewProcessResult("dotnet", 1, "", "not found", 0, errors.New("not found")), errors.New("not found")
		}
		return models.NewProcessResult(opts.Command, 0, "1.0.0", "", 0, nil), nil
	}

	doc := doctor.NewDoctor(mockRunner, logger.NopLogger())
	res, err := doc.RunDiagnostics(context.Background(), &buf, config.DefaultConfig())

	if err != nil {
		t.Fatalf("unexpected doctor error: %v", err)
	}

	if res.AllOK {
		t.Errorf("expected AllOK false when dotnet is missing")
	}
}
