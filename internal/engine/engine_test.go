package engine_test

import (
	"bytes"
	"context"
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
