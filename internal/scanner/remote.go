package scanner

import (
	"encoding/json"
	"path"
	"sort"
	"strings"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// DetectRemote finds buildable projects in a recursive repository file listing, applying the same
// rules as the detectors but without a checkout. Paths use forward slashes and directories end
// with "/". readFile fetches a file's content by its listed path and is only called for
// package.json files whose project type cannot be told from the file names alone, so a folder
// with both a Vite config and a "next" dependency is reported as React here.
func DetectRemote(paths []string, readFile func(filePath string) ([]byte, error)) []models.Project {
	filesByDir := make(map[string][]string)
	for _, p := range paths {
		p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
		if p == "" || strings.HasSuffix(p, "/") || insideSkippedDir(p) {
			continue
		}
		dir := path.Dir(p)
		if dir == "." {
			dir = ""
		}
		filesByDir[dir] = append(filesByDir[dir], path.Base(p))
	}

	dirs := make([]string, 0, len(filesByDir))
	for dir := range filesByDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	var projects []models.Project
	for _, dir := range dirs {
		projectType := remoteProjectType(dir, filesByDir[dir], readFile)
		if projectType == models.ProjectTypeUnknown {
			continue
		}
		name := path.Base(dir)
		if dir == "" {
			name = ""
		}
		projects = append(projects, models.Project{Name: name, Path: dir, Type: projectType})
	}
	return projects
}

func insideSkippedDir(filePath string) bool {
	parts := strings.Split(filePath, "/")
	for _, dir := range parts[:len(parts)-1] {
		if skipDirs[strings.ToLower(dir)] {
			return true
		}
	}
	return false
}

// remoteProjectType mirrors the detector order: Next.js, then React, then .NET.
func remoteProjectType(dir string, files []string, readFile func(filePath string) ([]byte, error)) models.ProjectType {
	hasPackageJSON, hasNextConfig, hasViteConfig, hasDotnet := false, false, false, false
	for _, f := range files {
		ext := strings.ToLower(path.Ext(f))
		switch {
		case f == "package.json":
			hasPackageJSON = true
		case strings.HasPrefix(f, "next.config."):
			hasNextConfig = true
		case strings.HasPrefix(f, "vite.config."):
			hasViteConfig = true
		case ext == ".csproj" || ext == ".sln":
			hasDotnet = true
		}
	}

	if hasNextConfig {
		return models.ProjectTypeNext
	}

	// A Vite config already settles the folder as buildable, so its package.json is not fetched.
	var deps map[string]bool
	if hasPackageJSON && !hasViteConfig && readFile != nil {
		if data, err := readFile(path.Join(dir, "package.json")); err == nil {
			var pkg struct {
				Dependencies map[string]string `json:"dependencies"`
				DevDeps      map[string]string `json:"devDependencies"`
			}
			if json.Unmarshal(data, &pkg) == nil {
				deps = make(map[string]bool)
				for dep := range pkg.Dependencies {
					deps[dep] = true
				}
				for dep := range pkg.DevDeps {
					deps[dep] = true
				}
			}
		}
	}

	switch {
	case deps["next"]:
		return models.ProjectTypeNext
	case hasViteConfig || deps["react"] || deps["vite"]:
		return models.ProjectTypeReact
	case hasDotnet:
		return models.ProjectTypeDotnet
	}
	return models.ProjectTypeUnknown
}
