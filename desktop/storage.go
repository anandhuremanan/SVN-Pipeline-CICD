package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/depcache"
	"github.com/anandhuremanan/cicd-engine/internal/engine"
	"github.com/anandhuremanan/cicd-engine/internal/storage/builds"
	"github.com/anandhuremanan/cicd-engine/internal/storage/logs"
)

const (
	// lowWorkspaceBytes is the free space below which the app warns that builds may not fit.
	lowWorkspaceBytes = 2 << 30
	// minWorkspaceBytes is the free space below which a build is not started at all.
	minWorkspaceBytes = 500 << 20
	// minDataBytes is the free space the app needs to keep its history and settings.
	minDataBytes = 20 << 20

	// portableSettingsFile is used next to the executable when the user settings folder cannot be written.
	portableSettingsFile = "svn-builder-settings.json"
)

// DriveInfo describes an internal drive the app could work on.
type DriveInfo struct {
	// Root is the drive root, e.g. "D:\".
	Root string `json:"root"`
	// Folder is the working folder the app would use on this drive, e.g. "D:\SvnBuilder".
	Folder     string  `json:"folder"`
	FreeBytes  float64 `json:"freeBytes"`
	TotalBytes float64 `json:"totalBytes"`
}

// StorageInfo tells the UI where the app keeps its working files and how much room is left.
type StorageInfo struct {
	// Folder is the working folder chosen in Settings; empty means automatic.
	Folder string `json:"folder"`
	// WorkspaceDir is where projects are downloaded and built.
	WorkspaceDir       string  `json:"workspaceDir"`
	WorkspaceFreeBytes float64 `json:"workspaceFreeBytes"`
	// DataDir is where the build history and logs are kept.
	DataDir       string  `json:"dataDir"`
	DataFreeBytes float64 `json:"dataFreeBytes"`
	// AutoWorkspaceDir and AutoDataDir are the locations used when no folder is chosen.
	AutoWorkspaceDir       string  `json:"autoWorkspaceDir"`
	AutoWorkspaceFreeBytes float64 `json:"autoWorkspaceFreeBytes"`
	AutoDataDir            string  `json:"autoDataDir"`
	AutoDataFreeBytes      float64 `json:"autoDataFreeBytes"`
	// Drives lists the internal drives that can be chosen.
	Drives []DriveInfo `json:"drives"`
	// Warning is set when one of the locations is nearly full.
	Warning string `json:"warning"`
	// SavedPackages is the number of projects whose installed packages are kept for faster builds.
	SavedPackages int `json:"savedPackages"`
	// PackageKeepDays is how long they are kept after a project's last build; 0 means until cleared.
	PackageKeepDays int `json:"packageKeepDays"`
}

const (
	// defaultPackageKeepDays applies until a choice is made in Settings.
	defaultPackageKeepDays = 7
	// keepPackagesForever is the stored value for "until I clear them".
	keepPackagesForever = -1
)

// packageKeepDays returns how long unused saved packages are kept; 0 means forever. Callers hold a.mu.
func (a *App) packageKeepDays() int {
	switch days := a.settings.PackageKeepDays; {
	case days == keepPackagesForever:
		return 0
	case days > 0:
		return days
	}
	return defaultPackageKeepDays
}

// prunePackages deletes, in the background, the saved packages of projects that have not been
// built within the time chosen in Settings. Callers hold a.mu.
func (a *App) prunePackages() {
	days := a.packageKeepDays()
	if days == 0 || a.workspaceDir == "" {
		return
	}
	cache := depcache.New(filepath.Join(a.workspaceDir, engine.DependencyCacheDir))

	a.cleanup.Add(1)
	go func() {
		defer a.cleanup.Done()
		cache.Prune(time.Duration(days) * 24 * time.Hour)
	}()
}

// SetPackageKeepDays chooses how long saved packages are kept after a project's last build.
// Zero or less means until they are cleared by hand.
func (a *App) SetPackageKeepDays(days int) (StorageInfo, error) {
	a.mu.Lock()
	if days <= 0 {
		a.settings.PackageKeepDays = keepPackagesForever
	} else {
		a.settings.PackageKeepDays = days
	}
	saveErr := a.saveSettings()
	if !a.running {
		a.prunePackages()
	}
	a.mu.Unlock()

	if saveErr != nil {
		return a.GetStorage(), fmt.Errorf("The change works until you close the app, but it could not be remembered: %v", saveErr)
	}
	return a.GetStorage(), nil
}

// ClearSavedPackages deletes the packages kept from earlier builds. The next build of each
// project installs everything again.
func (a *App) ClearSavedPackages() (StorageInfo, error) {
	a.mu.Lock()
	running := a.running
	workspaceDir := a.workspaceDir
	a.mu.Unlock()
	if running {
		return a.GetStorage(), fmt.Errorf("Wait for the running build to finish before clearing saved packages.")
	}

	// Renamed at once, deleted in the background: the count drops to zero immediately.
	saved := filepath.Join(workspaceDir, engine.DependencyCacheDir)
	doomed := fmt.Sprintf("%s-%d%s", saved, time.Now().UnixNano(), discardedSuffix)
	if err := os.Rename(saved, doomed); err != nil {
		if os.IsNotExist(err) {
			return a.GetStorage(), nil
		}
		return a.GetStorage(), fmt.Errorf("The saved packages are in use and could not be cleared. Try again in a moment.")
	}

	a.cleanup.Add(1)
	go func() {
		defer a.cleanup.Done()
		discardWorkspace(workspaceDir, doomed)
	}()
	return a.GetStorage(), nil
}

// configureStorage points the build engine at a working folder. An empty folder means automatic:
// projects are built in the Windows temp folder and the history is kept with the user settings.
func (a *App) configureStorage(folder string) error {
	workspaceDir := automaticWorkspaceDir()
	dataDir := a.configDir
	if folder != "" {
		workspaceDir = filepath.Join(folder, "workspaces")
		dataDir = folder
	}

	buildStore, err := builds.NewFileStore(filepath.Join(dataDir, "builds"))
	if err != nil {
		return err
	}
	logStore, err := logs.NewFileLogStore(filepath.Join(dataDir, "logs"))
	if err != nil {
		return err
	}

	out := &lineWriter{emit: func(line string) { a.emit(eventLog, line) }}
	a.engine = engine.NewEngine(out, nil, nil, nil, nil, nil, nil, nil, buildStore, &stageLogStore{inner: logStore, app: a}, nil)
	a.workspaceDir = workspaceDir
	a.dataDir = dataDir
	return nil
}

// discardedSuffix marks a temporary download that is finished with and only waiting to be deleted.
const discardedSuffix = ".delete"

// discardWorkspace deletes the temporary download of a finished build. It is renamed first, so
// that a deletion cut short (the app closing, the PC shutting down) can be completed on the next
// start. Nothing outside the app's own workspace folder is ever touched.
func discardWorkspace(workspaceRoot, path string) {
	rel, err := filepath.Rel(workspaceRoot, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || strings.ContainsAny(rel, `\/`) {
		return
	}

	doomed := path
	if !strings.HasSuffix(path, discardedSuffix) {
		doomed = path + discardedSuffix
		if err := os.Rename(path, doomed); err != nil {
			doomed = path
		}
	}

	if os.RemoveAll(doomed) == nil {
		return
	}
	// Read-only files (SVN keeps some) block removal on Windows; make them writable and retry.
	_ = filepath.WalkDir(doomed, func(p string, _ os.DirEntry, walkErr error) error {
		if walkErr == nil {
			_ = os.Chmod(p, 0777)
		}
		return nil
	})
	_ = os.RemoveAll(doomed)
}

// purgeWorkspaces deletes every temporary download left in workspaceRoot by an earlier session:
// ones whose deletion was cut short, and ones orphaned by a crash or a forced close. It is only
// safe because a single copy of the app runs at a time (see SingleInstanceLock in main.go) and
// it is called before any build of this session starts.
func purgeWorkspaces(workspaceRoot string) {
	entries, err := os.ReadDir(workspaceRoot)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() && (strings.HasPrefix(entry.Name(), workspacePrefix) || strings.HasSuffix(entry.Name(), discardedSuffix)) {
			discardWorkspace(workspaceRoot, filepath.Join(workspaceRoot, entry.Name()))
		}
	}
}

// workspacePrefix is how the engine names the temporary download of a build ("build-<time>-<id>").
const workspacePrefix = "build-"

// automaticWorkspaceDir is where builds happen when no working folder is chosen in Settings.
func automaticWorkspaceDir() string {
	return filepath.Join(os.TempDir(), appDirName)
}

// GetStorage reports where the app works and which drives it could use instead.
func (a *App) GetStorage() StorageInfo {
	a.mu.Lock()
	info := StorageInfo{Folder: a.settings.StorageDir, WorkspaceDir: a.workspaceDir, DataDir: a.dataDir, Drives: []DriveInfo{}}
	info.PackageKeepDays = a.packageKeepDays()
	a.mu.Unlock()

	workspaceFree, _, workspaceKnown := diskSpace(info.WorkspaceDir)
	dataFree, _, dataKnown := diskSpace(info.DataDir)
	info.WorkspaceFreeBytes = float64(workspaceFree)
	info.DataFreeBytes = float64(dataFree)
	info.SavedPackages = depcache.New(filepath.Join(info.WorkspaceDir, engine.DependencyCacheDir)).Count()

	info.AutoWorkspaceDir = filepath.Join(os.TempDir(), appDirName)
	info.AutoDataDir = a.configDir
	if free, _, ok := diskSpace(info.AutoWorkspaceDir); ok {
		info.AutoWorkspaceFreeBytes = float64(free)
	}
	if free, _, ok := diskSpace(info.AutoDataDir); ok {
		info.AutoDataFreeBytes = float64(free)
	}

	for _, root := range fixedDriveRoots() {
		free, total, ok := diskSpace(root)
		if !ok {
			continue
		}
		info.Drives = append(info.Drives, DriveInfo{
			Root:       root,
			Folder:     filepath.Join(root, appDirName),
			FreeBytes:  float64(free),
			TotalBytes: float64(total),
		})
	}

	switch {
	case workspaceKnown && workspaceFree < lowWorkspaceBytes:
		info.Warning = fmt.Sprintf("Drive %s, where projects are downloaded and built, has only %s free.", driveName(info.WorkspaceDir), formatBytes(workspaceFree))
	case dataKnown && dataFree < minDataBytes:
		info.Warning = fmt.Sprintf("Drive %s is full, so the build history and your settings can't be saved.", driveName(info.DataDir))
	}
	return info
}

// SetStorageDir moves the app's working files to folder. An empty folder switches back to automatic.
func (a *App) SetStorageDir(folder string) (StorageInfo, error) {
	folder = strings.TrimSpace(folder)

	a.mu.Lock()
	running := a.running
	a.mu.Unlock()
	if running {
		return a.GetStorage(), fmt.Errorf("Wait for the running build to finish before changing this.")
	}

	if folder != "" {
		folder = filepath.Clean(folder)
		if !filepath.IsAbs(folder) {
			return a.GetStorage(), fmt.Errorf("Choose a full folder path such as D:\\SvnBuilder.")
		}
		if err := os.MkdirAll(folder, 0755); err != nil {
			return a.GetStorage(), fmt.Errorf("The folder %s can't be created. Choose another drive or folder.", folder)
		}
		probe, err := os.MkdirTemp(folder, ".svnbuilder-check-")
		if err != nil {
			return a.GetStorage(), fmt.Errorf("You don't have permission to save files in %s.", folder)
		}
		_ = os.Remove(probe)
	}

	a.mu.Lock()
	previous := a.settings.StorageDir
	if err := a.configureStorage(folder); err != nil {
		a.mu.Unlock()
		return a.GetStorage(), fmt.Errorf("The folder %s can't be used: %v", folder, err)
	}
	a.settings.StorageDir = folder
	saveErr := a.saveSettings()
	a.mu.Unlock()

	if saveErr != nil {
		return a.GetStorage(), fmt.Errorf("The change works until you close the app, but it could not be remembered (it was %q before): %v", previous, saveErr)
	}
	return a.GetStorage(), nil
}

// settingsPaths lists where settings may live: the user settings folder first, then next to the
// executable, which is used when the first cannot be written (for example on a full drive).
func (a *App) settingsPaths() []string {
	paths := []string{filepath.Join(a.configDir, "settings.json")}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), portableSettingsFile))
	}
	return paths
}

// loadSettings reads the most recently saved settings file that is intact.
func (a *App) loadSettings() Settings {
	var newest Settings
	var newestTime int64

	for _, path := range a.settingsPaths() {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var s Settings
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		if modified := info.ModTime().UnixNano(); modified > newestTime {
			newest, newestTime = s, modified
		}
	}
	return newest
}

// saveSettings writes a.settings to the first location that accepts it. Callers hold a.mu.
func (a *App) saveSettings() error {
	data, err := json.MarshalIndent(a.settings, "", "  ")
	if err != nil {
		return err
	}

	var lastErr error
	for _, path := range a.settingsPaths() {
		if lastErr = writeFileAtomic(path, data); lastErr == nil {
			return nil
		}
	}
	return lastErr
}

// writeFileAtomic replaces path only once the new content is safely on disk, so a full drive
// cannot leave an empty or half-written file behind.
func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func driveName(path string) string {
	if volume := filepath.VolumeName(path); volume != "" {
		return volume
	}
	return path
}

func formatBytes(n uint64) string {
	const gb = 1 << 30
	if n >= gb {
		return fmt.Sprintf("%.1f GB", float64(n)/gb)
	}
	return fmt.Sprintf("%d MB", n>>20)
}
