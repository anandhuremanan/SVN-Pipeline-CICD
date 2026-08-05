package builder_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/builder"
	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/process"
)

type captureEventListener struct {
	events []builder.BuildEvent
}

func (c *captureEventListener) OnBuildEvent(event builder.BuildEvent) {
	c.events = append(c.events, event)
}

func TestBuilderRegistry(t *testing.T) {
	mockRunner := process.NewMockRunner()
	reg := builder.NewDefaultRegistry(mockRunner, logger.NopLogger())

	reactProj := models.Project{Name: "client", Type: models.ProjectTypeReact}
	b, err := reg.BuilderFor(reactProj)
	if err != nil || b.Name() != "React Builder" {
		t.Errorf("expected React Builder, got %v (err: %v)", b, err)
	}

	nextProj := models.Project{Name: "admin", Type: models.ProjectTypeNext}
	b, err = reg.BuilderFor(nextProj)
	if err != nil || b.Name() != "Next.js Builder" {
		t.Errorf("expected Next.js Builder, got %v (err: %v)", b, err)
	}

	dotnetProj := models.Project{Name: "api", Type: models.ProjectTypeDotnet}
	b, err = reg.BuilderFor(dotnetProj)
	if err != nil || b.Name() != ".NET Builder" {
		t.Errorf("expected .NET Builder, got %v (err: %v)", b, err)
	}
}

func TestReactBuilderExecution(t *testing.T) {
	tempDir := t.TempDir()
	// Create pnpm-lock.yaml to test lockfile detection
	_ = os.WriteFile(filepath.Join(tempDir, "pnpm-lock.yaml"), []byte(""), 0644)

	var executedCmds []string
	mockRunner := process.NewMockRunner()
	mockRunner.RunFunc = func(ctx context.Context, opts models.ExecOptions, handlers ...process.StreamHandler) (models.ProcessResult, error) {
		cmdStr := opts.Command
		if len(opts.Args) > 0 {
			cmdStr += " " + opts.Args[0]
		}
		executedCmds = append(executedCmds, cmdStr)
		return models.NewProcessResult(opts.Command, 0, "success", "", 0, nil), nil
	}

	reactBuilder := builder.NewReactBuilder(mockRunner, logger.NopLogger())
	proj := models.Project{Name: "client", Type: models.ProjectTypeReact, RootDir: tempDir}

	listener := &captureEventListener{}
	res, err := reactBuilder.Build(context.Background(), proj, listener)

	if err != nil {
		t.Fatalf("unexpected react build error: %v", err)
	}

	if res.Status != models.BuildStatusSuccess {
		t.Errorf("expected build status SUCCESS, got %s", res.Status)
	}

	if len(res.Steps) != 2 {
		t.Fatalf("expected 2 build steps, got %d", len(res.Steps))
	}

	if executedCmds[0] != "pnpm install" {
		t.Errorf("expected pnpm install, got %s", executedCmds[0])
	}

	if len(listener.events) == 0 {
		t.Error("expected build events to be captured")
	}
}

func TestDotnetBuilderExecution(t *testing.T) {
	tempDir := t.TempDir()

	mockRunner := process.NewMockRunner()
	dotnetBuilder := builder.NewDotnetBuilder(mockRunner, logger.NopLogger())
	proj := models.Project{Name: "api", Type: models.ProjectTypeDotnet, RootDir: tempDir}

	res, err := dotnetBuilder.Build(context.Background(), proj, nil)
	if err != nil {
		t.Fatalf("unexpected dotnet build error: %v", err)
	}

	if res.Status != models.BuildStatusSuccess {
		t.Errorf("expected build status SUCCESS, got %s", res.Status)
	}

	if len(res.Steps) != 2 || res.Steps[0].Name != "Restore" || res.Steps[1].Name != "Publish" {
		t.Errorf("unexpected steps in dotnet build: %+v", res.Steps)
	}
}

func TestReactBuilderDefaultScript(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempDir, "package-lock.json"), []byte("{}"), 0644)

	var executedArgs []string
	mockRunner := process.NewMockRunner()
	mockRunner.RunFunc = func(ctx context.Context, opts models.ExecOptions, handlers ...process.StreamHandler) (models.ProcessResult, error) {
		executedArgs = append(executedArgs, opts.Args...)
		return models.NewProcessResult(opts.Command, 0, "ok", "", 0, nil), nil
	}

	proj := models.Project{Name: "client", Type: models.ProjectTypeReact, RootDir: tempDir}
	listener := &captureEventListener{}
	_, err := builder.NewReactBuilder(mockRunner, logger.NopLogger()).Build(context.Background(), proj, listener)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify default "build" script was used
	found := false
	for _, a := range executedArgs {
		if a == "build" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'build' in executed args, got: %v", executedArgs)
	}

	// Verify EventInfo event was emitted with "(default)" label
	infoFound := false
	for _, ev := range listener.events {
		if ev.Type == builder.EventInfo && contains(ev.Message, "(default)") {
			infoFound = true
		}
	}
	if !infoFound {
		t.Error("expected EventInfo event with '(default)' label")
	}
}

func TestReactBuilderCustomScript(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempDir, "package-lock.json"), []byte("{}"), 0644)

	var executedArgs []string
	mockRunner := process.NewMockRunner()
	mockRunner.RunFunc = func(ctx context.Context, opts models.ExecOptions, handlers ...process.StreamHandler) (models.ProcessResult, error) {
		executedArgs = append(executedArgs, opts.Args...)
		return models.NewProcessResult(opts.Command, 0, "ok", "", 0, nil), nil
	}

	proj := models.Project{
		Name:    "dashboard",
		Type:    models.ProjectTypeReact,
		RootDir: tempDir,
		Build:   models.BuildConfig{Script: "deploy"},
	}
	listener := &captureEventListener{}
	_, err := builder.NewReactBuilder(mockRunner, logger.NopLogger()).Build(context.Background(), proj, listener)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify "deploy" script was used instead of "build"
	deployFound := false
	for _, a := range executedArgs {
		if a == "deploy" {
			deployFound = true
		}
	}
	if !deployFound {
		t.Errorf("expected 'deploy' in executed args, got: %v", executedArgs)
	}

	// Verify EventInfo event was emitted with "(from cicd.yaml)" label
	infoFound := false
	for _, ev := range listener.events {
		if ev.Type == builder.EventInfo && contains(ev.Message, "(from cicd.yaml)") {
			infoFound = true
		}
	}
	if !infoFound {
		t.Error("expected EventInfo event with '(from cicd.yaml)' label")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

