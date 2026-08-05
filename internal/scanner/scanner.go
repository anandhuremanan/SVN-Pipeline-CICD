package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// Scanner defines the interface for recursive project discovery.
type Scanner interface {
	// Scan recursively searches rootDir for supported projects.
	Scan(ctx context.Context, rootDir string) ([]models.Project, error)
}

// RepositoryScanner implements Scanner using a collection of registered Detectors.
type RepositoryScanner struct {
	detectors []Detector
	logger    logger.Logger
}

// NewRepositoryScanner constructs a RepositoryScanner with default detectors.
func NewRepositoryScanner(log logger.Logger, customDetectors ...Detector) *RepositoryScanner {
	if log == nil {
		log = logger.NopLogger()
	}

	detectors := customDetectors
	if len(detectors) == 0 {
		// Ordered: Next.js detector must run before React detector
		detectors = []Detector{
			NewNextDetector(),
			NewReactDetector(),
			NewDotnetDetector(),
		}
	}

	return &RepositoryScanner{
		detectors: detectors,
		logger:    log,
	}
}

// Scan recursively walks rootDir and evaluates registered detectors.
func (s *RepositoryScanner) Scan(ctx context.Context, rootDir string) ([]models.Project, error) {
	s.logger.Info("starting repository project scan", "root", rootDir)

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("repository scanner: invalid root path %q: %w", rootDir, err)
	}

	var projects []models.Project

	skipDirs := map[string]bool{
		"node_modules": true,
		".svn":         true,
		".git":         true,
		"bin":          true,
		"obj":          true,
		"dist":         true,
		".next":        true,
		".workspaces":  true,
	}

	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			dirName := d.Name()
			if skipDirs[strings.ToLower(dirName)] {
				return filepath.SkipDir
			}

			entries, err := readDirEntryNames(path)
			if err != nil {
				return nil
			}

			relPath, err := filepath.Rel(absRoot, path)
			if err != nil {
				relPath = path
			}
			if relPath == "." {
				relPath = ""
			}

			for _, detector := range s.detectors {
				proj, found, err := detector.Detect(ctx, path, relPath, entries)
				if err != nil {
					s.logger.Warn("detector error", "detector", detector.Name(), "path", path, "err", err)
					continue
				}
				if found && proj != nil {
					s.logger.Info("detected project", "name", proj.Name, "type", proj.Type, "path", proj.Path)
					projects = append(projects, *proj)
					break
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("repository scanner: failed walking directory %q: %w", absRoot, err)
	}

	return projects, nil
}

func readDirEntryNames(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	return names, nil
}
