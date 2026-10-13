package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/anandhuremanan/cicd-engine/internal/release"
)

// waitForPIDEnv tells a freshly started copy to wait for the copy it replaces to exit first, so
// the single-instance lock is free when it starts.
const waitForPIDEnv = "SVNBUILDER_WAIT_FOR_PID"

// AppInfo identifies the running build.
type AppInfo struct {
	// Version is the release number, or "dev" for a build made outside a release.
	Version string `json:"version"`
	// UpdateSource is the folder this build looks in for new versions; empty when it never does.
	UpdateSource string `json:"updateSource"`
}

// UpdateInfo is the result of looking for a new version.
type UpdateInfo struct {
	Available bool   `json:"available"`
	Version   string `json:"version"`
	Notes     string `json:"notes"`
	// Problem says why the check could not be completed, in words for the user.
	Problem string `json:"problem"`
}

// GetAppInfo returns the version of the running app.
func (a *App) GetAppInfo() AppInfo {
	info := AppInfo{Version: version}
	if isReleaseBuild() {
		info.UpdateSource = updateSource
	}
	return info
}

// CheckForUpdate looks in the release folder for a version newer than this one. Only a manifest
// signed by the publisher counts.
func (a *App) CheckForUpdate() UpdateInfo {
	if !isReleaseBuild() || updateSource == "" {
		return UpdateInfo{Problem: "This is a development build, which is updated by hand."}
	}

	manifest, err := release.ReadManifest(filepath.Join(updateSource, release.ManifestFile))
	if err != nil {
		if os.IsNotExist(err) {
			return UpdateInfo{Problem: fmt.Sprintf("No release was found in %s.", updateSource)}
		}
		return UpdateInfo{Problem: fmt.Sprintf("The release folder %s can't be read. Check your network or VPN connection.", updateSource)}
	}
	if err := manifest.Verify(updatePublicKey); err != nil {
		return UpdateInfo{Problem: "A release was found, but it was not published by the makers of this app, so it was ignored."}
	}

	if !release.Newer(manifest.Version, version) {
		return UpdateInfo{Version: version}
	}
	return UpdateInfo{Available: true, Version: manifest.Version, Notes: manifest.Notes}
}

// InstallUpdate replaces this program with the newest release and restarts it.
func (a *App) InstallUpdate() error {
	a.mu.Lock()
	running := a.running
	a.mu.Unlock()
	if running {
		return fmt.Errorf("Wait for the running build to finish before updating.")
	}

	check := a.CheckForUpdate()
	if !check.Available {
		if check.Problem != "" {
			return fmt.Errorf("%s", check.Problem)
		}
		return fmt.Errorf("This is already the latest version.")
	}

	manifest, err := release.ReadManifest(filepath.Join(updateSource, release.ManifestFile))
	if err != nil || manifest.Verify(updatePublicKey) != nil {
		return fmt.Errorf("The release could not be read again. Try once more.")
	}
	// The file name comes from the manifest; never let it point outside the release folder.
	if manifest.File != filepath.Base(manifest.File) {
		return fmt.Errorf("The release is not valid.")
	}

	current, err := os.Executable()
	if err != nil {
		return fmt.Errorf("The app could not find its own file: %v", err)
	}
	current, _ = filepath.EvalSymlinks(current)

	manual := fmt.Sprintf("Copy %s from %s by hand instead.", manifest.File, updateSource)

	// Download next to the current file so the final swap is a rename on one drive.
	incoming := current + ".new"
	if err := copyFile(filepath.Join(updateSource, manifest.File), incoming); err != nil {
		_ = os.Remove(incoming)
		return fmt.Errorf("The new version could not be saved in %s. %s", filepath.Dir(current), manual)
	}

	// The signature covers this checksum, so a file that matches it is exactly what was published.
	sum, err := release.FileSHA256(incoming)
	if err != nil || !strings.EqualFold(sum, manifest.SHA256) {
		_ = os.Remove(incoming)
		return fmt.Errorf("The downloaded file does not match the published release, so it was not installed. Try again.")
	}

	// Windows lets a running program be renamed, though not overwritten: move it aside, put the
	// new one in its place. The old file is deleted the next time the app starts.
	previous := current + ".old"
	_ = os.Remove(previous)
	if err := os.Rename(current, previous); err != nil {
		_ = os.Remove(incoming)
		return fmt.Errorf("The app's file could not be replaced. %s", manual)
	}
	if err := os.Rename(incoming, current); err != nil {
		_ = os.Rename(previous, current)
		_ = os.Remove(incoming)
		return fmt.Errorf("The app's file could not be replaced. %s", manual)
	}

	next := exec.Command(current)
	next.Env = append(os.Environ(), waitForPIDEnv+"="+strconv.Itoa(os.Getpid()))
	if err := next.Start(); err != nil {
		return fmt.Errorf("The update is installed. Close the app and open it again to use it.")
	}

	runtime.Quit(a.ctx)
	return nil
}

// finishUpdate runs at program start: it waits for the copy being replaced to exit and removes
// the file an earlier update left behind.
func finishUpdate() {
	if pid, err := strconv.Atoi(os.Getenv(waitForPIDEnv)); err == nil && pid > 0 {
		waitForProcessExit(pid)
		_ = os.Unsetenv(waitForPIDEnv)
	}

	if current, err := os.Executable(); err == nil {
		_ = os.Remove(current + ".old")
		_ = os.Remove(current + ".new")
	}
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
