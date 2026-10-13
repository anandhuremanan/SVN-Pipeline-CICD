// Package release defines the manifest that announces a new version of the desktop app, and how
// it is signed and checked. A release folder holds the app's executable and a latest.json next
// to it. The manifest is signed with a private key that stays with whoever publishes releases;
// the app carries the matching public key, so a file placed in the folder by anyone else is
// rejected even though the folder itself is an ordinary share.
package release

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ManifestFile is the name of the manifest inside a release folder.
const ManifestFile = "latest.json"

// Manifest describes the newest published version.
type Manifest struct {
	Version string `json:"version"`
	// File is the executable's name inside the release folder.
	File string `json:"file"`
	// SHA256 is the hex checksum of File.
	SHA256 string `json:"sha256"`
	// Notes is a short description of what changed, shown to the user.
	Notes string `json:"notes,omitempty"`
	// Signature is the base64 Ed25519 signature over the version and checksum.
	Signature string `json:"signature"`
}

// signedMessage is what the signature covers: the version and the exact file content.
func (m Manifest) signedMessage() []byte {
	return []byte(m.Version + "\n" + strings.ToLower(m.SHA256))
}

// Sign fills in the manifest's signature.
func (m *Manifest) Sign(privateKey ed25519.PrivateKey) {
	m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, m.signedMessage()))
}

// Verify checks the signature against a base64 public key.
func (m Manifest) Verify(publicKeyBase64 string) error {
	publicKey, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("release: this app has no valid update key")
	}
	signature, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || !ed25519.Verify(publicKey, m.signedMessage(), signature) {
		return fmt.Errorf("release: the update is not signed by the publisher of this app")
	}
	return nil
}

// ReadManifest loads a manifest file.
func ReadManifest(path string) (Manifest, error) {
	var m Manifest
	data, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("release: %s is not a valid manifest: %w", path, err)
	}
	if m.Version == "" || m.File == "" || m.SHA256 == "" {
		return m, fmt.Errorf("release: %s is incomplete", path)
	}
	return m, nil
}

// FileSHA256 returns the hex SHA-256 checksum of a file.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Newer reports whether version a is later than version b. Versions are dotted numbers such as
// "1.4.0"; a leading "v" is ignored and missing parts count as zero.
func Newer(a, b string) bool {
	pa, pb := parts(a), parts(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func parts(version string) []int {
	var numbers []int
	for _, field := range strings.Split(strings.TrimPrefix(strings.TrimSpace(version), "v"), ".") {
		n, err := strconv.Atoi(field)
		if err != nil {
			n = 0
		}
		numbers = append(numbers, n)
	}
	return numbers
}
