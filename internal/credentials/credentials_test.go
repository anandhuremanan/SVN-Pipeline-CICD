package credentials_test

import (
	"os"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/credentials"
)

func TestStaticProvider(t *testing.T) {
	provider := credentials.NewStaticProvider("admin", "secret123")

	if provider.Username() != "admin" {
		t.Errorf("expected username admin, got %s", provider.Username())
	}

	pass, err := provider.Password()
	if err != nil {
		t.Fatalf("unexpected error getting password: %v", err)
	}
	if pass != "secret123" {
		t.Errorf("expected password secret123, got %s", pass)
	}
}

func TestEnvironmentProvider(t *testing.T) {
	os.Setenv("TEST_SVN_USER", "envuser")
	os.Setenv("TEST_SVN_PASS", "envpass")
	defer func() {
		os.Unsetenv("TEST_SVN_USER")
		os.Unsetenv("TEST_SVN_PASS")
	}()

	provider := credentials.NewEnvironmentProvider("TEST_SVN_USER", "TEST_SVN_PASS")

	if provider.Username() != "envuser" {
		t.Errorf("expected username envuser, got %s", provider.Username())
	}

	pass, err := provider.Password()
	if err != nil {
		t.Fatalf("unexpected error getting password: %v", err)
	}
	if pass != "envpass" {
		t.Errorf("expected password envpass, got %s", pass)
	}
}

func TestEnvironmentProviderMissingPassword(t *testing.T) {
	os.Unsetenv("TEST_MISSING_PASS")

	provider := credentials.NewEnvironmentProvider("TEST_MISSING_USER", "TEST_MISSING_PASS")

	_, err := provider.Password()
	if err == nil {
		t.Error("expected error for missing environment variable password, got nil")
	}
}
