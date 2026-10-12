package publisher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

const (
	buildDirPrefix = "Build-"
	// BuildInfoFile is the summary file written into every numbered build folder.
	BuildInfoFile = "build-info.json"
)

// BuildDirName returns the folder name used for a build number (e.g. "Build-0007").
func BuildDirName(number int) string {
	return fmt.Sprintf("%s%04d", buildDirPrefix, number)
}

// AllocateBuildDir reserves the next free build number under root and creates its folder.
// The folder is claimed with a single non-recursive mkdir, so two machines publishing to the
// same network share at the same time can never receive the same number.
func AllocateBuildDir(root string) (int, string, error) {
	if strings.TrimSpace(root) == "" {
		return 0, "", fmt.Errorf("publisher: output folder cannot be empty")
	}

	if err := os.MkdirAll(root, 0755); err != nil {
		return 0, "", fmt.Errorf("publisher: cannot reach output folder %q: %w", root, err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, "", fmt.Errorf("publisher: cannot read output folder %q: %w", root, err)
	}

	highest := 0
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), buildDirPrefix) {
			continue
		}
		if n, convErr := strconv.Atoi(strings.TrimPrefix(entry.Name(), buildDirPrefix)); convErr == nil && n > highest {
			highest = n
		}
	}

	for attempt := 0; attempt < 1000; attempt++ {
		number := highest + 1 + attempt
		dir := filepath.Join(root, BuildDirName(number))
		err := os.Mkdir(dir, 0755)
		if err == nil {
			return number, dir, nil
		}
		if !os.IsExist(err) {
			return 0, "", fmt.Errorf("publisher: failed to create build folder %q: %w", dir, err)
		}
	}

	return 0, "", fmt.Errorf("publisher: could not reserve a build number under %q", root)
}

// BuildInfo is the schema of build-info.json stored next to the published projects.
type BuildInfo struct {
	BuildNumber int                `json:"buildNumber"`
	Repository  string             `json:"repository"`
	Revision    string             `json:"revision"`
	BuiltBy     string             `json:"builtBy,omitempty"`
	Machine     string             `json:"machine,omitempty"`
	StartedAt   string             `json:"startedAt"`
	PublishedAt string             `json:"publishedAt"`
	Projects    []BuildInfoProject `json:"projects"`
}

// BuildInfoProject describes one published project inside a build folder.
type BuildInfoProject struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Folder string `json:"folder"`
}

// WriteBuildInfo writes build-info.json into the job's build folder.
func WriteBuildInfo(job models.Job, builtBy string) error {
	if job.BuildDir == "" {
		return fmt.Errorf("publisher: job has no build folder")
	}

	machine, _ := os.Hostname()
	info := BuildInfo{
		BuildNumber: job.BuildNumber,
		Repository:  job.Repository,
		Revision:    job.Revision,
		BuiltBy:     builtBy,
		Machine:     machine,
		StartedAt:   job.StartedAt.Format(time.RFC3339),
		PublishedAt: time.Now().Format(time.RFC3339),
		Projects:    []BuildInfoProject{},
	}

	for _, bRes := range job.BuildResults {
		pubDir, ok := job.DeploymentOutputs[bRes.Project.Name]
		if !ok {
			continue
		}
		info.Projects = append(info.Projects, BuildInfoProject{
			Name:   bRes.Project.Name,
			Type:   string(bRes.Project.Type),
			Folder: filepath.Base(pubDir),
		})
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("publisher: failed to marshal build info: %w", err)
	}

	infoPath := filepath.Join(job.BuildDir, BuildInfoFile)
	if err := os.WriteFile(infoPath, data, 0644); err != nil {
		return fmt.Errorf("publisher: failed to write %s: %w", infoPath, err)
	}
	return nil
}
