package builder_test

import (
	"context"
	"strings"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/builder"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/process"
)

type recordingListener struct {
	events []builder.BuildEvent
}

func (r *recordingListener) OnBuildEvent(event builder.BuildEvent) {
	r.events = append(r.events, event)
}

func TestInstallStepReportsPackageProgressAndLiveOutput(t *testing.T) {
	var installEnv []string

	runner := process.NewMockRunner()
	runner.RunFunc = func(ctx context.Context, opts models.ExecOptions, handlers ...process.StreamHandler) (models.ProcessResult, error) {
		if len(handlers) != 1 {
			t.Fatalf("expected each step to run with a stream handler, got %d", len(handlers))
		}
		if opts.Args[0] == "install" {
			installEnv = opts.Env
			handlers[0].Stderr("npm http fetch GET 200 https://registry.npmjs.org/react 120ms (cache miss)")
			handlers[0].Stderr("npm http fetch GET 200 https://registry.npmjs.org/vite 98ms (cache miss)")
			handlers[0].Stderr("npm warn deprecated left-pad@1.0.0")
			stderr := "npm http fetch GET 200 https://registry.npmjs.org/react 120ms (cache miss)\nnpm warn deprecated left-pad@1.0.0\n"
			return models.NewProcessResult(opts.Command, 0, "added 2 packages", stderr, 0, nil), nil
		}
		handlers[0].Stdout("vite v8.2.0 building for production...")
		return models.NewProcessResult(opts.Command, 0, "", "", 0, nil), nil
	}

	listener := &recordingListener{}
	project := models.Project{Name: "Client", Path: "Client", Type: models.ProjectTypeReact, RootDir: t.TempDir()}

	result, err := builder.NewReactBuilder(runner, nil).Build(context.Background(), project, listener)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	if len(installEnv) != 1 || installEnv[0] != "npm_config_loglevel=http" {
		t.Errorf("expected npm install to be asked for fetch logging, got env %v", installEnv)
	}

	var progress, output []string
	for _, ev := range listener.events {
		switch ev.Type {
		case builder.EventStepProgress:
			progress = append(progress, ev.Step+": "+ev.Message)
		case builder.EventStepOutput:
			output = append(output, ev.Step+": "+ev.Message)
		}
	}

	if len(progress) != 1 || progress[0] != "Install Dependencies: Downloaded 2 packages" {
		t.Errorf("expected one progress summary for 2 packages, got %v", progress)
	}
	wantOutput := []string{"Install Dependencies: npm warn deprecated left-pad@1.0.0", "Build: vite v8.2.0 building for production..."}
	if strings.Join(output, "|") != strings.Join(wantOutput, "|") {
		t.Errorf("expected live output %v, got %v", wantOutput, output)
	}

	// Fetch lines are progress noise and must not end up in the stored step output.
	if stderr := result.Steps[0].StdErr; strings.Contains(stderr, "http fetch") || !strings.Contains(stderr, "npm warn deprecated") {
		t.Errorf("expected stored stderr without fetch lines but with the warning, got %q", stderr)
	}
}
