package publisher

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// Publisher defines the contract for copying deployable build artifacts into permanent deployment directories.
type Publisher interface {
	Publish(ctx context.Context, repoName string, project models.Project, result models.BuildResult, deploymentDir string, replaceExisting bool) (string, error)
}

// DefaultPublisher implements Publisher without framework coupling.
type DefaultPublisher struct {
	log logger.Logger
}

// NewDefaultPublisher creates a new DefaultPublisher instance.
func NewDefaultPublisher(log logger.Logger) *DefaultPublisher {
	if log == nil {
		log = logger.NopLogger()
	}
	return &DefaultPublisher{log: log}
}

// Publish copies project artifacts into deploymentDir/repoName/project.Name.
func (p *DefaultPublisher) Publish(ctx context.Context, repoName string, project models.Project, result models.BuildResult, deploymentDir string, replaceExisting bool) (string, error) {
	if len(result.Artifacts) == 0 {
		return "", fmt.Errorf("publisher: no deployable artifacts found for project %q", project.Name)
	}

	projName := project.Name
	if projName == "" {
		projName = "app"
	}

	var targetDir string
	if deploymentDir != "" && strings.HasSuffix(filepath.Clean(deploymentDir), filepath.Clean(projName)) {
		targetDir = deploymentDir
	} else if repoName != "" && repoName != "." && repoName != "/" {
		targetDir = filepath.Join(deploymentDir, repoName, projName)
	} else {
		targetDir = filepath.Join(deploymentDir, projName)
	}

	if _, err := os.Stat(targetDir); err == nil {
		if replaceExisting {
			p.log.Info("removing existing deployment directory", "dir", targetDir)
			if err := os.RemoveAll(targetDir); err != nil {
				return "", fmt.Errorf("publisher: failed to clean existing deployment directory %q: %w", targetDir, err)
			}
		} else {
			return "", fmt.Errorf("publisher: deployment destination %q already exists and replaceExisting is false", targetDir)
		}
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("publisher: failed to create deployment directory %q: %w", targetDir, err)
	}

	for _, art := range result.Artifacts {
		info, err := os.Stat(art.Path)
		if err != nil {
			p.log.Warn("artifact path not found", "path", art.Path, "err", err)
			continue
		}

		if info.IsDir() {
			if err := copyDirContents(art.Path, targetDir); err != nil {
				return "", fmt.Errorf("publisher: failed copying artifact directory %q to %q: %w", art.Path, targetDir, err)
			}
		} else {
			destFile := filepath.Join(targetDir, filepath.Base(art.Path))
			if err := copyFile(art.Path, destFile); err != nil {
				return "", fmt.Errorf("publisher: failed copying artifact file %q to %q: %w", art.Path, destFile, err)
			}
		}
	}

	p.log.Info("successfully published deployable output", "project", project.Name, "target", targetDir)
	return targetDir, nil
}

func copyDirContents(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
			if err := copyDirContents(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
