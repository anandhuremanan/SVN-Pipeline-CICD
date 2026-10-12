package engine_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/builder"
	"github.com/anandhuremanan/cicd-engine/internal/cli"
	"github.com/anandhuremanan/cicd-engine/internal/credentials"
	"github.com/anandhuremanan/cicd-engine/internal/engine"
	"github.com/anandhuremanan/cicd-engine/internal/environment"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/process"
	"github.com/anandhuremanan/cicd-engine/internal/publisher"
	"github.com/anandhuremanan/cicd-engine/internal/repository"
)

type mockEnvValidator struct{}

func (m *mockEnvValidator) Validate(ctx context.Context) (environment.EnvironmentInfo, error) {
	tools := map[string]environment.ToolInfo{
		"svn":    {Name: "SVN", Installed: true, Version: "1.14.2"},
		"node":   {Name: "Node", Installed: true, Version: "v22.15.0"},
		"npm":    {Name: "NPM", Installed: true, Version: "10.9.0"},
		"dotnet": {Name: "Dotnet", Installed: true, Version: "9.0.100"},
	}
	return environment.EnvironmentInfo{Tools: tools}, nil
}

type mockSCMProvider struct{}

func (m *mockSCMProvider) Name() string { return "MockSCM" }

func (m *mockSCMProvider) CheckoutOrUpdate(ctx context.Context, repoURL, targetDir, revision string, creds credentials.Provider) (*repository.Info, error) {
	_ = os.MkdirAll(targetDir, 0755)
	return &repository.Info{
		URL:       repoURL,
		Revision:  "100",
		TargetDir: targetDir,
	}, nil
}

func (m *mockSCMProvider) GetRevision(ctx context.Context, targetDir string) (string, error) {
	return "100", nil
}

type mockScanner struct{}

func (m *mockScanner) Scan(ctx context.Context, rootDir string) ([]models.Project, error) {
	return []models.Project{
		{Name: "Client", Path: "src/client", Type: models.ProjectTypeReact, RootDir: filepath.Join(rootDir, "src/client")},
		{Name: "Admin", Path: "src/admin", Type: models.ProjectTypeNext, RootDir: filepath.Join(rootDir, "src/admin")},
		{Name: "API", Path: "src/api", Type: models.ProjectTypeDotnet, RootDir: filepath.Join(rootDir, "src/api")},
	}, nil
}

func TestEngineRunBuildPlanFlow(t *testing.T) {
	var out bytes.Buffer

	mockRunner := process.NewMockRunner()
	mockRegistry := builder.NewDefaultRegistry(mockRunner, nil)
	mockSCM := &mockSCMProvider{}

	eng := engine.NewEngine(&out, mockRunner, &mockEnvValidator{}, mockSCM, nil, &mockScanner{}, mockRegistry, nil, nil, nil, nil)

	req := cli.RunRequest{
		RepositoryURL: "https://svn.example.com/repo/trunk",
		KeepWorkspace: false,
	}

	err := eng.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected engine run error: %v", err)
	}

	output := out.String()

	expectedSubstrings := []string{
		"Checking environment...",
		"✓ SVN 1.14.2",
		"✓ Node v22.15.0",
		"✓ NPM 10.9.0",
		"✓ Dotnet 9.0.100",
		"Creating workspace...",
		"Checking out repository...",
		"Sanitizing workspace...",
		"Scanning repository...",
		"Found 3 project(s).",
		"PROJECT: Client",
		"Step [1/2]: Install Dependencies",
		"Command:   npm install",
		"✓ Install Dependencies",
		"✓ Build",
		"PROJECT: Admin",
		"PROJECT: API",
		"Step [1/2]: Restore",
		"Command:   dotnet restore",
		"✓ Restore",
		"✓ Build",
		"Build Summary",
		"Revision:   100",
		"Succeeded:  3",
		"Build completed successfully.",
	}

	for _, expected := range expectedSubstrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain %q, but got:\n%s", expected, output)
		}
	}
}

func TestEngineRunPublishesToNumberedOutputDir(t *testing.T) {
	var out bytes.Buffer

	// Every command run inside a project folder leaves behind the output folder its builder looks for.
	mockRunner := process.NewMockRunner()
	mockRunner.RunFunc = func(ctx context.Context, opts models.ExecOptions, handlers ...process.StreamHandler) (models.ProcessResult, error) {
		if opts.Dir != "" {
			for _, outDir := range []string{"dist", ".next", "publish"} {
				_ = os.MkdirAll(filepath.Join(opts.Dir, outDir), 0755)
				_ = os.WriteFile(filepath.Join(opts.Dir, outDir, "output.txt"), []byte("built"), 0644)
			}
		}
		return models.NewProcessResult(opts.Command, 0, "mock stdout", "", 0, nil), nil
	}
	mockRegistry := builder.NewDefaultRegistry(mockRunner, nil)

	eng := engine.NewEngine(&out, mockRunner, &mockEnvValidator{}, &mockSCMProvider{}, nil, &mockScanner{}, mockRegistry, nil, nil, nil, nil)

	outputDir := filepath.Join(t.TempDir(), "share")
	workspaceRoot := filepath.Join(t.TempDir(), "ws")

	for want := 1; want <= 2; want++ {
		var finished models.Job
		req := cli.RunRequest{
			RepositoryURL:  "https://svn.example.com/repos/UAM/MY-APP/trunk",
			Username:       "anandhu",
			OutputDir:      outputDir,
			WorkspaceRoot:  workspaceRoot,
			CleanWorkspace: true,
			OnFinish:       func(job models.Job) { finished = job },
		}

		if err := eng.Run(context.Background(), req); err != nil {
			t.Fatalf("run %d: unexpected engine run error: %v\n%s", want, err, out.String())
		}

		if finished.BuildNumber != want {
			t.Fatalf("run %d: expected build number %d, got %d", want, want, finished.BuildNumber)
		}

		expectedDir := filepath.Join(outputDir, "MY-APP", publisher.BuildDirName(want))
		if finished.BuildDir != expectedDir {
			t.Errorf("run %d: expected build dir %q, got %q", want, expectedDir, finished.BuildDir)
		}

		for _, project := range []string{"Client", "Admin", "API"} {
			if _, err := os.Stat(filepath.Join(expectedDir, project, "output.txt")); err != nil {
				t.Errorf("run %d: expected published output for %s in %s: %v", want, project, expectedDir, err)
			}
		}

		if _, err := os.Stat(filepath.Join(expectedDir, publisher.BuildInfoFile)); err != nil {
			t.Errorf("run %d: expected %s in build dir: %v", want, publisher.BuildInfoFile, err)
		}

		if !strings.HasPrefix(finished.Workspace, workspaceRoot) {
			t.Errorf("run %d: expected workspace under %q, got %q", want, workspaceRoot, finished.Workspace)
		}
		if _, err := os.Stat(finished.Workspace); !os.IsNotExist(err) {
			t.Errorf("run %d: expected workspace %q to be removed after the build", want, finished.Workspace)
		}
	}
}

func TestEngineRunBuildsOnlyRequestedFolderWithScript(t *testing.T) {
	var out bytes.Buffer
	var commands []string

	mockRunner := process.NewMockRunner()
	mockRunner.RunFunc = func(ctx context.Context, opts models.ExecOptions, handlers ...process.StreamHandler) (models.ProcessResult, error) {
		commands = append(commands, opts.Command+" "+strings.Join(opts.Args, " "))
		if opts.Dir != "" {
			for _, outDir := range []string{"dist", ".next"} {
				_ = os.MkdirAll(filepath.Join(opts.Dir, outDir), 0755)
				_ = os.WriteFile(filepath.Join(opts.Dir, outDir, "output.txt"), []byte("built"), 0644)
			}
		}
		return models.NewProcessResult(opts.Command, 0, "mock stdout", "", 0, nil), nil
	}
	mockRegistry := builder.NewDefaultRegistry(mockRunner, nil)

	eng := engine.NewEngine(&out, mockRunner, &mockEnvValidator{}, &mockSCMProvider{}, nil, &mockScanner{}, mockRegistry, nil, nil, nil, nil)

	var finished models.Job
	req := cli.RunRequest{
		RepositoryURL:  "https://svn.example.com/repos/UAM/MY-APP/trunk",
		OutputDir:      filepath.Join(t.TempDir(), "share"),
		WorkspaceRoot:  filepath.Join(t.TempDir(), "ws"),
		CleanWorkspace: true,
		ProjectPaths:   []string{"src/admin", `SRC\Client\`},
		BuildScript:    "deploy",
		ProjectScripts: map[string]string{"src/admin": "build:prod"},
		OnFinish:       func(job models.Job) { finished = job },
	}

	if err := eng.Run(context.Background(), req); err != nil {
		t.Fatalf("unexpected engine run error: %v\n%s", err, out.String())
	}

	// Projects are built in the order the folders were requested, not in scan order.
	if len(finished.BuildResults) != 2 || finished.BuildResults[0].Project.Name != "Admin" || finished.BuildResults[1].Project.Name != "Client" {
		t.Fatalf("expected Admin then Client to be built, got %+v", finished.BuildResults)
	}
	if len(finished.DeploymentOutputs) != 2 {
		t.Errorf("expected Admin and Client to be published, got %v", finished.DeploymentOutputs)
	}

	joined := strings.Join(commands, "\n")
	if !strings.Contains(joined, "npm run build:prod") {
		t.Errorf("expected the script set for src/admin to be run, commands were:\n%s", joined)
	}
	if !strings.Contains(joined, "npm run deploy") {
		t.Errorf("expected the run-wide script to be used for Client, commands were:\n%s", joined)
	}
	if strings.Contains(joined, "dotnet") {
		t.Errorf("expected no dotnet build when src/api was not requested, commands were:\n%s", joined)
	}
}

func TestEngineRunUnknownFolderFails(t *testing.T) {
	var out bytes.Buffer

	mockRunner := process.NewMockRunner()
	eng := engine.NewEngine(&out, mockRunner, &mockEnvValidator{}, &mockSCMProvider{}, nil, &mockScanner{}, builder.NewDefaultRegistry(mockRunner, nil), nil, nil, nil, nil)

	outputDir := filepath.Join(t.TempDir(), "share")
	err := eng.Run(context.Background(), cli.RunRequest{
		RepositoryURL:  "https://svn.example.com/repos/UAM/MY-APP/trunk",
		OutputDir:      outputDir,
		WorkspaceRoot:  filepath.Join(t.TempDir(), "ws"),
		CleanWorkspace: true,
		ProjectPaths:   []string{"src/api", "Clinet"},
	})

	var filterErr *engine.ProjectFilterError
	if !errors.As(err, &filterErr) {
		t.Fatalf("expected ProjectFilterError, got %v", err)
	}
	if len(filterErr.Requested) != 1 || filterErr.Requested[0] != "Clinet" {
		t.Errorf("expected only the unmatched folder to be reported, got %v", filterErr.Requested)
	}
	if len(filterErr.Available) != 3 || filterErr.Available[0] != "src/client" {
		t.Errorf("expected the available folders to be listed, got %v", filterErr.Available)
	}
	if entries, _ := os.ReadDir(outputDir); len(entries) != 0 {
		t.Errorf("expected nothing in the output folder after a failed run, found %d entries", len(entries))
	}
}

func TestEngineInvalidConfigPath(t *testing.T) {
	var out bytes.Buffer
	eng := engine.NewEngine(&out, nil, &mockEnvValidator{}, &mockSCMProvider{}, nil, &mockScanner{}, nil, nil, nil, nil, nil)

	req := cli.RunRequest{
		RepositoryURL: "https://svn.example.com/repo/trunk",
		ConfigPath:    "/invalid/config/path.json",
	}

	err := eng.Run(context.Background(), req)
	if err == nil {
		t.Error("expected error for invalid config path, got nil")
	}
}
