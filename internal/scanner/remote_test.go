package scanner_test

import (
	"fmt"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/scanner"
)

func TestDetectRemote(t *testing.T) {
	paths := []string{
		"Client/",
		"Client/package.json",
		"Client/vite.config.ts",
		"Client/src/",
		"Client/src/main.tsx",
		"Client/node_modules/react/package.json",
		"Admin/package.json",
		"Admin/next.config.js",
		"Portal/package.json",
		"Tools/package.json",
		"Api/Api.csproj",
		"Api/Program.cs",
		"Api/bin/Debug/Old.csproj",
		"docs/readme.md",
	}

	contents := map[string]string{
		"Client/package.json": `{"dependencies": {"next": "15"}}`,
		"Portal/package.json": `{"devDependencies": {"vite": "8"}}`,
		"Tools/package.json":  `{"dependencies": {"lodash": "4"}}`,
	}
	readFile := func(filePath string) ([]byte, error) {
		if content, ok := contents[filePath]; ok {
			return []byte(content), nil
		}
		return nil, fmt.Errorf("not found: %s", filePath)
	}

	got := scanner.DetectRemote(paths, readFile)

	want := []struct {
		path string
		typ  models.ProjectType
	}{
		{"Admin", models.ProjectTypeNext},
		{"Api", models.ProjectTypeDotnet},
		// A Vite config settles the type without reading package.json.
		{"Client", models.ProjectTypeReact},
		{"Portal", models.ProjectTypeReact},
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d projects, got %d: %+v", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i].Path != w.path || got[i].Type != w.typ {
			t.Errorf("project %d: expected %s (%s), got %s (%s)", i, w.path, w.typ, got[i].Path, got[i].Type)
		}
	}
}

func TestDetectRemoteTopFolderProject(t *testing.T) {
	got := scanner.DetectRemote([]string{"App.sln", "src/", "src/Program.cs"}, nil)

	if len(got) != 1 || got[0].Path != "" || got[0].Type != models.ProjectTypeDotnet {
		t.Fatalf("expected one .NET project in the top folder, got %+v", got)
	}
}
