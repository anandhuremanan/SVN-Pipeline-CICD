package release_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/release"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.0.1", "1.0.0", true},
		{"1.10.0", "1.9.9", true},
		{"2.0", "1.99.99", true},
		{"v1.2.0", "1.2", false},
		{"1.0.0", "1.0.1", false},
		{"1.0.0", "1.0.0", false},
		{"1.0.0", "dev", true},
	}
	for _, c := range cases {
		if got := release.Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestSignedManifestIsAcceptedOnlyUnchangedAndFromThePublisher(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	otherPublic, _, _ := ed25519.GenerateKey(nil)
	encode := base64.StdEncoding.EncodeToString

	m := release.Manifest{Version: "1.2.0", File: "svn-builder.exe", SHA256: "ABCDEF0123"}
	m.Sign(privateKey)

	if err := m.Verify(encode(publicKey)); err != nil {
		t.Fatalf("expected the publisher's manifest to verify: %v", err)
	}
	if err := m.Verify(encode(otherPublic)); err == nil {
		t.Error("expected a manifest signed with another key to be rejected")
	}

	swapped := m
	swapped.SHA256 = "0000000000"
	if err := swapped.Verify(encode(publicKey)); err == nil {
		t.Error("expected a manifest pointing at a different file to be rejected")
	}

	renumbered := m
	renumbered.Version = "9.9.9"
	if err := renumbered.Verify(encode(publicKey)); err == nil {
		t.Error("expected a manifest with a changed version to be rejected")
	}

	if err := m.Verify(""); err == nil {
		t.Error("expected verification to fail when the app has no update key")
	}
}
