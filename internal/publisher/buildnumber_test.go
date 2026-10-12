package publisher_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/publisher"
)

func TestAllocateBuildDirIsSequential(t *testing.T) {
	root := filepath.Join(t.TempDir(), "share", "MY-APP")

	for want := 1; want <= 3; want++ {
		number, dir, err := publisher.AllocateBuildDir(root)
		if err != nil {
			t.Fatalf("unexpected allocation error: %v", err)
		}
		if number != want {
			t.Errorf("expected build number %d, got %d", want, number)
		}
		if dir != filepath.Join(root, publisher.BuildDirName(want)) {
			t.Errorf("unexpected build dir %q for number %d", dir, want)
		}
		if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
			t.Errorf("expected build dir %q to exist", dir)
		}
	}
}

func TestAllocateBuildDirContinuesAfterHighestExisting(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "Build-0041"), 0755)
	_ = os.MkdirAll(filepath.Join(root, "Build-0007"), 0755)
	_ = os.MkdirAll(filepath.Join(root, "notes"), 0755)
	_ = os.WriteFile(filepath.Join(root, "Build-0099"), []byte("a file, not a build"), 0644)

	number, _, err := publisher.AllocateBuildDir(root)
	if err != nil {
		t.Fatalf("unexpected allocation error: %v", err)
	}
	if number != 42 {
		t.Errorf("expected build number 42, got %d", number)
	}
}

func TestAllocateBuildDirConcurrentCallersGetUniqueNumbers(t *testing.T) {
	root := t.TempDir()

	const callers = 20
	numbers := make([]int, callers)
	errs := make([]error, callers)

	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			numbers[i], _, errs[i] = publisher.AllocateBuildDir(root)
		}(i)
	}
	wg.Wait()

	seen := make(map[int]bool)
	for i, n := range numbers {
		if errs[i] != nil {
			t.Fatalf("caller %d: unexpected allocation error: %v", i, errs[i])
		}
		if seen[n] {
			t.Errorf("build number %d was handed out twice", n)
		}
		seen[n] = true
	}
}
