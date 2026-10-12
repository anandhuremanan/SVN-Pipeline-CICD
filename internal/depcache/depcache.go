// Package depcache keeps a Node project's installed dependencies (node_modules) between builds.
// A fresh checkout otherwise reinstalls tens of thousands of files every time. The folder is moved,
// not copied: into the project before its build and back out afterwards, which is instant as long
// as the cache and the workspace are on the same drive.
package depcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

const (
	modulesDir = "node_modules"
	infoFile   = "info.json"
)

// Cache stores one node_modules folder per project under root.
type Cache struct {
	root string
}

// Info records which project a cached folder belongs to.
type Info struct {
	Repository string `json:"repository"`
	Project    string `json:"project"`
	Path       string `json:"path"`
	SavedAt    string `json:"savedAt"`
}

// New returns a cache rooted at root. The folder is created on first save.
func New(root string) *Cache {
	return &Cache{root: root}
}

// Supports reports whether dependencies of this project type can be kept.
func Supports(project models.Project) bool {
	return project.Type == models.ProjectTypeReact || project.Type == models.ProjectTypeNext
}

// key identifies a project by its repository address and its folder inside the repository.
func key(repoURL string, project models.Project) string {
	path := strings.ToLower(strings.Trim(strings.ReplaceAll(project.Path, "\\", "/"), "/"))
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimRight(repoURL, "/")) + "|" + path))
	return hex.EncodeToString(sum[:10])
}

func (c *Cache) entryDir(repoURL string, project models.Project) string {
	return filepath.Join(c.root, key(repoURL, project))
}

// Restore moves the saved node_modules of project into its checkout. It reports whether anything
// was restored; a project that already has a node_modules folder is left as it is.
func (c *Cache) Restore(repoURL string, project models.Project) (bool, error) {
	saved := filepath.Join(c.entryDir(repoURL, project), modulesDir)
	if info, err := os.Stat(saved); err != nil || !info.IsDir() {
		return false, nil
	}

	target := filepath.Join(project.RootDir, modulesDir)
	if _, err := os.Lstat(target); err == nil {
		return false, nil
	}

	if err := rename(saved, target); err != nil {
		return false, fmt.Errorf("depcache: cannot restore saved packages for %s: %w", project.Name, err)
	}
	return true, nil
}

// Save moves the node_modules of project out of its checkout and keeps it for the next build.
func (c *Cache) Save(repoURL string, project models.Project) error {
	source := filepath.Join(project.RootDir, modulesDir)
	if info, err := os.Stat(source); err != nil || !info.IsDir() {
		return nil
	}

	entry := c.entryDir(repoURL, project)
	saved := filepath.Join(entry, modulesDir)
	if err := os.RemoveAll(saved); err != nil {
		return fmt.Errorf("depcache: cannot replace saved packages for %s: %w", project.Name, err)
	}
	if err := os.MkdirAll(entry, 0755); err != nil {
		return fmt.Errorf("depcache: cannot create %s: %w", entry, err)
	}
	if err := rename(source, saved); err != nil {
		return fmt.Errorf("depcache: cannot save packages for %s: %w", project.Name, err)
	}

	info := Info{Repository: repoURL, Project: project.Name, Path: project.Path, SavedAt: time.Now().Format(time.RFC3339)}
	if data, err := json.MarshalIndent(info, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(entry, infoFile), data, 0644)
	}
	return nil
}

// Drop deletes the saved packages of project, so that its next build installs from scratch.
func (c *Cache) Drop(repoURL string, project models.Project) error {
	return os.RemoveAll(c.entryDir(repoURL, project))
}

// Count returns how many projects have saved packages.
func (c *Cache) Count() int {
	entries, err := os.ReadDir(c.root)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if info, err := os.Stat(filepath.Join(c.root, entry.Name(), modulesDir)); err == nil && info.IsDir() {
			count++
		}
	}
	return count
}

// Prune deletes the saved packages of projects that have not been built for longer than maxAge
// and returns how many were removed. A project's age is counted from its last build.
func (c *Cache) Prune(maxAge time.Duration) int {
	entries, err := os.ReadDir(c.root)
	if err != nil || maxAge <= 0 {
		return 0
	}

	removed := 0
	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(c.root, entry.Name())
		saved, err := os.Stat(filepath.Join(dir, modulesDir))
		if err != nil {
			// Nothing kept here right now; a build may be using it.
			continue
		}

		// info.json is rewritten on every save, so its time is the last build of the project.
		lastUsed := saved.ModTime()
		if info, err := os.Stat(filepath.Join(dir, infoFile)); err == nil {
			lastUsed = info.ModTime()
		}
		if lastUsed.Before(cutoff) && os.RemoveAll(dir) == nil {
			removed++
		}
	}
	return removed
}

// rename moves a folder, retrying briefly: on Windows a virus scanner that still has a file open
// inside makes the first attempts fail.
func rename(from, to string) error {
	var err error
	for attempt := 0; attempt < 6; attempt++ {
		if err = os.Rename(from, to); err == nil {
			return nil
		}
		time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
	}
	return err
}
