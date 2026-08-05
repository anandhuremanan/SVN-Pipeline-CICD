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

// ReactDetector discovers React (Vite) web application projects.
type ReactDetector struct{}

// NewReactDetector constructs a ReactDetector instance.
func NewReactDetector() *ReactDetector {
	return &ReactDetector{}
}

func (d *ReactDetector) Name() string {
	return "React (Vite)"
}

func (d *ReactDetector) Detect(ctx context.Context, projectDir string, relPath string, entries []string) (*models.Project, bool, error) {
	hasPackageJSON := slices.Contains(entries, "package.json")
	hasViteConfig := false

	for _, entry := range entries {
		if strings.HasPrefix(entry, "vite.config.") {
			hasViteConfig = true
			break
		}
	}

	if !hasPackageJSON && !hasViteConfig {
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

	if hasViteConfig {
		return &models.Project{
			Name:    name,
			Path:    relPath,
			Type:    models.ProjectTypeReact,
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

		_, hasReact := pkg.Dependencies["react"]
		_, hasVite := pkg.DevDeps["vite"]
		if !hasReact {
			_, hasReact = pkg.DevDeps["react"]
		}
		if !hasVite {
			_, hasVite = pkg.Dependencies["vite"]
		}

		if hasReact || hasVite {
			return &models.Project{
				Name:    name,
				Path:    relPath,
				Type:    models.ProjectTypeReact,
				RootDir: projectDir,
			}, true, nil
		}
	}

	return nil, false, nil
}
