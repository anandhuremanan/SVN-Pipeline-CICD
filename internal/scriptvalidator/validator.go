// Package scriptvalidator validates npm/pnpm/yarn build scripts before any build begins.
// It reads package.json directly — without invoking npm — to confirm that the requested
// script exists. This surfaces invalid script names during pipeline validation rather than
// at build time, producing clear, actionable diagnostics.
package scriptvalidator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// ValidationError describes a script configuration problem for a single project.
type ValidationError struct {
	Project          string
	ConfiguredScript string
	AvailableScripts []string
	Reason           string
}

func (e *ValidationError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Pipeline Validation Failed\n\n")
	fmt.Fprintf(&sb, "Project:\n%s\n\n", e.Project)
	fmt.Fprintf(&sb, "Configured script:\n%s\n\n", e.ConfiguredScript)
	if len(e.AvailableScripts) > 0 {
		fmt.Fprintf(&sb, "Available scripts:\n")
		for _, s := range e.AvailableScripts {
			fmt.Fprintf(&sb, "  %s\n", s)
		}
	}
	if e.Reason != "" {
		fmt.Fprintf(&sb, "\n%s", e.Reason)
	}
	return sb.String()
}

// packageJSON is the minimal subset of package.json that we need.
type packageJSON struct {
	Scripts map[string]string `json:"scripts"`
}

// readScripts parses package.json in the project root and returns the scripts map.
func readScripts(rootDir string) (map[string]string, error) {
	pkgPath := filepath.Join(rootDir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read package.json at %s: %w", pkgPath, err)
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("cannot parse package.json at %s: %w", pkgPath, err)
	}

	if pkg.Scripts == nil {
		return map[string]string{}, nil
	}
	return pkg.Scripts, nil
}

// sortedKeys returns alphabetically sorted keys from a map.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// isNodeProject returns true for React and Next.js project types.
func isNodeProject(pt models.ProjectType) bool {
	return pt == models.ProjectTypeReact || pt == models.ProjectTypeNext
}

// ResolveScript returns the effective script name for a project.
// If project.Build.Script is set, it is returned unchanged.
// Otherwise the default "build" is returned.
func ResolveScript(project models.Project) string {
	if project.Build.Script != "" {
		return project.Build.Script
	}
	return "build"
}

// IsOverridden reports whether the project has an explicit script override in YAML.
func IsOverridden(project models.Project) bool {
	return project.Build.Script != ""
}

// ValidateAll checks every Node project in the list.
// It returns the first ValidationError encountered, or nil if all scripts are valid.
func ValidateAll(projects []models.Project) error {
	for _, p := range projects {
		if !isNodeProject(p.Type) {
			continue
		}
		if p.RootDir == "" {
			continue
		}

		script := ResolveScript(p)

		scripts, err := readScripts(p.RootDir)
		if err != nil {
			// package.json missing or unreadable — let the builder surface the real error
			continue
		}

		if _, ok := scripts[script]; !ok {
			return &ValidationError{
				Project:          p.Name,
				ConfiguredScript: script,
				AvailableScripts: sortedKeys(scripts),
			}
		}
	}
	return nil
}
