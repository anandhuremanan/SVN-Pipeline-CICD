package depcache_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/depcache"
	"github.com/anandhuremanan/cicd-engine/internal/models"
)

const repoURL = "https://svn.example.com/repos/TPP"

func checkout(t *testing.T, root, name string) models.Project {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	return models.Project{Name: "client", Path: "PMJ-JBY/Client", Type: models.ProjectTypeReact, RootDir: dir}
}

func TestSaveThenRestoreMovesNodeModules(t *testing.T) {
	root := t.TempDir()
	cache := depcache.New(filepath.Join(root, ".deps"))

	first := checkout(t, root, "build-1")
	if restored, err := cache.Restore(repoURL, first); err != nil || restored {
		t.Fatalf("expected nothing to restore on the first build, got restored=%v err=%v", restored, err)
	}

	_ = os.MkdirAll(filepath.Join(first.RootDir, "node_modules", "react"), 0755)
	_ = os.WriteFile(filepath.Join(first.RootDir, "node_modules", "react", "index.js"), []byte("react"), 0644)

	if err := cache.Save(repoURL, first); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(first.RootDir, "node_modules")); !os.IsNotExist(err) {
		t.Errorf("expected node_modules to be moved out of the checkout")
	}
	if cache.Count() != 1 {
		t.Errorf("expected 1 saved project, got %d", cache.Count())
	}

	// The next build is a different checkout of the same project, written with other slashes and case.
	second := checkout(t, root, "build-2")
	second.Path = `pmj-jby\client`
	restored, err := cache.Restore(repoURL+"/", second)
	if err != nil || !restored {
		t.Fatalf("expected the saved packages to be restored, got restored=%v err=%v", restored, err)
	}
	if _, err := os.Stat(filepath.Join(second.RootDir, "node_modules", "react", "index.js")); err != nil {
		t.Errorf("expected restored node_modules content: %v", err)
	}
	if cache.Count() != 0 {
		t.Errorf("expected the cache entry to be in use by the build, got count %d", cache.Count())
	}
}

func TestOtherProjectsAndDropAreSeparate(t *testing.T) {
	root := t.TempDir()
	cache := depcache.New(filepath.Join(root, ".deps"))

	client := checkout(t, root, "build-1")
	_ = os.MkdirAll(filepath.Join(client.RootDir, "node_modules"), 0755)
	if err := cache.Save(repoURL, client); err != nil {
		t.Fatal(err)
	}

	admin := checkout(t, root, "build-2")
	admin.Path = "PMJ-JBY/Admin"
	if restored, _ := cache.Restore(repoURL, admin); restored {
		t.Errorf("a different project must not receive these packages")
	}

	if err := cache.Drop(repoURL, client); err != nil {
		t.Fatal(err)
	}
	again := checkout(t, root, "build-3")
	if restored, _ := cache.Restore(repoURL, again); restored {
		t.Errorf("expected nothing to restore after the saved packages were dropped")
	}
}

func TestPruneRemovesOnlyProjectsNotBuiltRecently(t *testing.T) {
	root := t.TempDir()
	cacheRoot := filepath.Join(root, ".deps")
	cache := depcache.New(cacheRoot)

	recent := checkout(t, root, "build-1")
	_ = os.MkdirAll(filepath.Join(recent.RootDir, "node_modules"), 0755)
	if err := cache.Save(repoURL, recent); err != nil {
		t.Fatal(err)
	}

	stale := checkout(t, root, "build-2")
	stale.Path = "PMJ-JBY/Admin"
	_ = os.MkdirAll(filepath.Join(stale.RootDir, "node_modules"), 0755)
	if err := cache.Save(repoURL, stale); err != nil {
		t.Fatal(err)
	}

	// Age the second project's entry by ten days.
	entries, _ := os.ReadDir(cacheRoot)
	old := time.Now().Add(-10 * 24 * time.Hour)
	aged := 0
	for _, entry := range entries {
		data, _ := os.ReadFile(filepath.Join(cacheRoot, entry.Name(), "info.json"))
		if strings.Contains(string(data), "PMJ-JBY/Admin") {
			_ = os.Chtimes(filepath.Join(cacheRoot, entry.Name(), "info.json"), old, old)
			aged++
		}
	}
	if aged != 1 {
		t.Fatalf("expected to age exactly one entry, aged %d", aged)
	}

	if removed := cache.Prune(7 * 24 * time.Hour); removed != 1 {
		t.Errorf("expected 1 project to be pruned, got %d", removed)
	}
	if cache.Count() != 1 {
		t.Errorf("expected the recently built project to be kept, count is %d", cache.Count())
	}
	if removed := cache.Prune(0); removed != 0 {
		t.Errorf("expected no pruning when no age limit is set, got %d", removed)
	}
}

func TestSupportsOnlyNodeProjects(t *testing.T) {
	if !depcache.Supports(models.Project{Type: models.ProjectTypeNext}) || depcache.Supports(models.Project{Type: models.ProjectTypeDotnet}) {
		t.Errorf("expected only Node projects to be supported")
	}
}
