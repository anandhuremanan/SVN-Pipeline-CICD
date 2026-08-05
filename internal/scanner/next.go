package scanner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// NextDetector discovers Next.js web application projects.
type NextDetector struct{}

// NewNextDetector constructs a NextDetector instance.
func NewNextDetector() *NextDetector {
	return &NextDetector{}
}

func (d *NextDetector) Name() string {
	return "Next.js"
}

func (d *NextDetector) Detect(ctx context.Context, projectDir string, relPath string, entries []string) (*models.Project, bool, error) {
	hasPackageJSON := slices.Contains(entries, "package.json")
	hasNextConfig := false

	for _, entry := range entries {
		if strings.HasPrefix(entry, "next.config.") {
			hasNextConfig = true
			break
		}
	}

	if !hasPackageJSON && !hasNextConfig {
		return nil, false, nil
	}

	var pkgName string
	if hasPackageJSON {
		pkgPath := filepath.Join(projectDir, "package.json")
		if data, err := os.ReadFile(pkgPath); err == nil {
			var pkg struct {
				Name         string            `json:"name"`
				Dependencies map[string]string `json:"dependencies"`
				DevDeps      map[string]string `json:"devDependencies"`
			}
			if err := json.Unmarshal(data, &pkg); err == nil {
				pkgName = strings.TrimSpace(pkg.Name)
			}
		}
	}

	name := pkgName
	if name == "" && relPath != "" {
		name = filepath.Base(relPath)
	}

	if hasNextConfig {
		return &models.Project{
			Name:    name,
			Path:    relPath,
			Type:    models.ProjectTypeNext,
			RootDir: projectDir,
		}, true, nil
	}

	if hasPackageJSON {
		pkgPath := filepath.Join(projectDir, "package.json")
		data, err := os.ReadFile(pkgPath)
		if err != nil {
			return nil, false, nil
		}

		var pkg struct {
			Name         string            `json:"name"`
			Dependencies map[string]string `json:"dependencies"`
			DevDeps      map[string]string `json:"devDependencies"`
		}

		if err := json.Unmarshal(data, &pkg); err != nil {
			return nil, false, nil
		}

		_, inDeps := pkg.Dependencies["next"]
		_, inDevDeps := pkg.DevDeps["next"]

		if inDeps || inDevDeps {
			return &models.Project{
				Name:    name,
				Path:    relPath,
				Type:    models.ProjectTypeNext,
				RootDir: projectDir,
			}, true, nil
		}
	}

	return nil, false, nil
}
