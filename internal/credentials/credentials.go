package credentials

import (
	"errors"
	"os"
)

// Provider abstracts credential resolution for repository authentication.
type Provider interface {
	// Username returns the repository username.
	Username() string
	// Password returns the repository password or an error if resolution fails.
	Password() (string, error)
}

// EnvironmentProvider resolves credentials from environment variables.
type EnvironmentProvider struct {
	userEnvKey string
	passEnvKey string
}

// NewEnvironmentProvider constructs an EnvironmentProvider.
// If keys are empty, defaults to SVN_USERNAME and SVN_PASSWORD.
func NewEnvironmentProvider(userEnvKey, passEnvKey string) *EnvironmentProvider {
	if userEnvKey == "" {
		userEnvKey = "SVN_USERNAME"
	}
	if passEnvKey == "" {
		passEnvKey = "SVN_PASSWORD"
	}
	return &EnvironmentProvider{
		userEnvKey: userEnvKey,
		passEnvKey: passEnvKey,
	}
}

// Username returns the username environment variable value.
func (p *EnvironmentProvider) Username() string {
	return os.Getenv(p.userEnvKey)
}

// Password returns the password environment variable value or an error if missing.
func (p *EnvironmentProvider) Password() (string, error) {
	pass := os.Getenv(p.passEnvKey)
	if pass == "" {
		return "", errors.New("password environment variable is empty or not set")
	}
	return pass, nil
}

// StaticProvider supplies static in-memory credentials.
type StaticProvider struct {
	username string
	password string
}

// NewStaticProvider creates a StaticProvider with fixed username and password values.
func NewStaticProvider(username, password string) *StaticProvider {
	return &StaticProvider{
		username: username,
		password: password,
	}
}

// Username returns the configured static username.
func (p *StaticProvider) Username() string {
	return p.username
}

// Password returns the configured static password.
func (p *StaticProvider) Password() (string, error) {
	return p.password, nil
}
