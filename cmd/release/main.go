// Command release builds and publishes a new version of the SVN Builder desktop app.
//
//	go run ./cmd/release keygen
//	go run ./cmd/release -version 1.2.0 -to \\SERVER\Share\SvnBuilder -notes "What changed"
//
// keygen is run once. It creates the private key that signs releases (kept outside the
// repository) and writes the matching public key into desktop/update_key.go, which is built into
// the app. Publishing builds the app with the version and release folder baked in, signs it, and
// copies it to the release folder; installed copies then offer the update by themselves.
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/anandhuremanan/cicd-engine/internal/release"
)

const (
	exeName      = "svn-builder.exe"
	buildName    = "svn-builder-release.exe"
	keyFileName  = "svn-builder-release.key"
	publicKeySrc = "update_key.go"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "keygen" {
		return keygen()
	}

	fs := flag.NewFlagSet("release", flag.ContinueOnError)
	versionFlag := fs.String("version", "", "Version to publish, for example 1.2.0")
	to := fs.String("to", "", "Release folder that installed copies check, for example \\\\SERVER\\Share\\SvnBuilder")
	notes := fs.String("notes", "", "Short description of what changed, shown to users")
	out := fs.String("out", "", "Without -to: where the standalone build is written (default: release\\SvnBuilder-<version> in the repository)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if !regexp.MustCompile(`^\d+(\.\d+){1,3}$`).MatchString(*versionFlag) {
		return fmt.Errorf("-version must be numbers separated by dots, for example 1.2.0")
	}
	desktopDir, err := findDesktopDir()
	if err != nil {
		return err
	}

	// Without -to the result is a standalone build: it has a version but looks nowhere for
	// updates, and is handed out by hand.
	if strings.TrimSpace(*to) == "" {
		return buildStandalone(desktopDir, *versionFlag, *out)
	}

	releaseDir, err := filepath.Abs(*to)
	if err != nil {
		return err
	}

	privateKey, err := loadPrivateKey()
	if err != nil {
		return err
	}

	// Refuse to publish a version that is not newer than what is already there.
	if existing, readErr := release.ReadManifest(filepath.Join(releaseDir, release.ManifestFile)); readErr == nil {
		if !release.Newer(*versionFlag, existing.Version) {
			return fmt.Errorf("version %s is not newer than %s, which is already published in %s", *versionFlag, existing.Version, releaseDir)
		}
	}

	built, err := buildApp(desktopDir, *versionFlag, releaseDir)
	if err != nil {
		return err
	}
	defer os.Remove(built)

	sum, err := release.FileSHA256(built)
	if err != nil {
		return err
	}
	manifest := release.Manifest{Version: *versionFlag, File: exeName, SHA256: sum, Notes: *notes}
	manifest.Sign(privateKey)

	if err := os.MkdirAll(releaseDir, 0755); err != nil {
		return fmt.Errorf("cannot reach the release folder %s: %w", releaseDir, err)
	}

	// The executable goes first and the manifest last, so nobody is offered a version whose
	// file is still being copied. The executable is copied under a temporary name and renamed,
	// because someone may be running it straight from the folder.
	target := filepath.Join(releaseDir, exeName)
	staged := target + ".publishing"
	if err := copyFile(built, staged); err != nil {
		return fmt.Errorf("cannot copy the app to %s: %w", releaseDir, err)
	}
	_ = os.Remove(target + ".previous")
	if _, statErr := os.Stat(target); statErr == nil {
		if err := os.Rename(target, target+".previous"); err != nil {
			_ = os.Remove(staged)
			return fmt.Errorf("cannot replace %s: %w", target, err)
		}
	}
	if err := os.Rename(staged, target); err != nil {
		return fmt.Errorf("cannot put the new app in place: %w", err)
	}
	_ = os.Remove(target + ".previous")

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(releaseDir, release.ManifestFile), data, 0644); err != nil {
		return fmt.Errorf("cannot write the release manifest: %w", err)
	}

	fmt.Printf("\nPublished version %s to %s\n", *versionFlag, releaseDir)
	fmt.Printf("  %s  (sha256 %s)\n", exeName, sum)
	fmt.Println("Installed copies will offer this update the next time they start.")
	return nil
}

// buildApp builds the desktop app with its version, and its update folder when it has one, baked
// in. It returns the path of the built file, which the caller removes when done with it.
func buildApp(desktopDir, version, updateSource string) (string, error) {
	if err := setProductVersion(desktopDir, version); err != nil {
		return "", err
	}

	fmt.Printf("Building version %s...\n", version)
	ldflags := fmt.Sprintf("-X main.version=%s", version)
	if updateSource != "" {
		ldflags += fmt.Sprintf(" -X 'main.updateSource=%s'", updateSource)
	}
	build := exec.Command("wails", "build", "-o", buildName, "-ldflags", ldflags)
	build.Dir = desktopDir
	build.Env = installedGoEnv()
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		return "", fmt.Errorf("build failed: %w", err)
	}
	return filepath.Join(desktopDir, "build", "bin", buildName), nil
}

// setProductVersion records the version in wails.json, which is where the version shown in the
// file's Properties in Windows comes from.
func setProductVersion(desktopDir, version string) error {
	path := filepath.Join(desktopDir, "wails.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	info, _ := config["info"].(map[string]interface{})
	if info == nil {
		info = map[string]interface{}{}
	}
	info["productVersion"] = version
	config["info"] = info

	updated, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(updated, '\n'), 0644)
}

// buildStandalone writes a versioned build that is handed out by hand and never updates itself.
func buildStandalone(desktopDir, version, out string) error {
	outDir := out
	if strings.TrimSpace(outDir) == "" {
		outDir = filepath.Join(filepath.Dir(desktopDir), "release", "SvnBuilder-"+version)
	}
	outDir, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}

	built, err := buildApp(desktopDir, version, "")
	if err != nil {
		return err
	}
	defer os.Remove(built)

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("cannot create %s: %w", outDir, err)
	}
	target := filepath.Join(outDir, exeName)
	if err := copyFile(built, target); err != nil {
		return fmt.Errorf("cannot write %s: %w", target, err)
	}
	sum, err := release.FileSHA256(target)
	if err != nil {
		return err
	}

	fmt.Printf("\nBuilt standalone version %s\n", version)
	fmt.Printf("  %s  (sha256 %s)\n", target, sum)
	fmt.Println("This build does not update itself; hand out the file again for later versions.")
	return nil
}

// installedGoEnv returns an environment in which `go` means the Go that is installed on the PC.
// Wails 2.12 cannot generate bindings with newer toolchains. When this tool is started with
// `go run`, a downloaded toolchain has put itself first on PATH and into GOROOT, so setting
// GOTOOLCHAIN=local alone would still pick that newer one.
func installedGoEnv() []string {
	var env []string
	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(name) {
		case "GOROOT", "GOTOOLCHAIN":
			continue
		case "PATH":
			var kept []string
			for _, dir := range filepath.SplitList(value) {
				if !strings.Contains(filepath.ToSlash(dir), "golang.org/toolchain@") {
					kept = append(kept, dir)
				}
			}
			entry = name + "=" + strings.Join(kept, string(os.PathListSeparator))
		}
		env = append(env, entry)
	}
	return append(env, "GOTOOLCHAIN=local")
}

// keyPath is where the signing key lives: in the publisher's own profile, never in the repository.
func keyPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, keyFileName), nil
}

func loadPrivateKey() (ed25519.PrivateKey, error) {
	path, err := keyPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no signing key at %s. Run `go run ./cmd/release keygen` once, or copy the key there from the PC that made it", path)
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%s is not a valid signing key", path)
	}
	return ed25519.PrivateKey(key), nil
}

func keygen() error {
	path, err := keyPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("a signing key already exists at %s. Replacing it would stop every installed copy from accepting updates, so it is left alone", path)
	}

	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(privateKey)+"\n"), 0600); err != nil {
		return fmt.Errorf("cannot save the signing key to %s: %w", path, err)
	}

	desktopDir, err := findDesktopDir()
	if err != nil {
		return err
	}
	source := fmt.Sprintf(`package main

// updatePublicKey checks that a release was published by the holder of the matching private key.
// Generated by `+"`go run ./cmd/release keygen`"+`; the private key is not in this repository.
const updatePublicKey = %q
`, base64.StdEncoding.EncodeToString(publicKey))
	if err := os.WriteFile(filepath.Join(desktopDir, publicKeySrc), []byte(source), 0644); err != nil {
		return err
	}

	fmt.Printf("Signing key saved to %s\n", path)
	fmt.Println("Keep a backup of that file somewhere safe and do not share it.")
	fmt.Printf("Public key written to %s\n", filepath.Join(desktopDir, publicKeySrc))
	return nil
}

// findDesktopDir locates the desktop app's folder from the repository root or from inside it.
func findDesktopDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "desktop", "wails.json")
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Dir(candidate), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("run this from inside the repository (desktop/wails.json was not found)")
		}
		dir = parent
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
