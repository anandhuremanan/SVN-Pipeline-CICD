package process

import (
	"testing"
)

func TestExecutableResolverWindows(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"npm", "npm.cmd"},
		{"npx", "npx.cmd"},
		{"pnpm", "pnpm.cmd"},
		{"yarn", "yarn.cmd"},
		{"NPM", "npm.cmd"},
		{"svn", "svn"},
		{"node", "node"},
		{"dotnet", "dotnet"},
		{"git", "git"},
		{"go", "go"},
	}

	for _, tt := range tests {
		t.Run("Windows_"+tt.input, func(t *testing.T) {
			resolved := resolveExecutableForOS(tt.input, "windows")
			if resolved != tt.expected {
				t.Errorf("for %q on windows: expected %q, got %q", tt.input, tt.expected, resolved)
			}
		})
	}
}

func TestExecutableResolverLinuxAndMacOS(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"npm", "npm"},
		{"npx", "npx"},
		{"pnpm", "pnpm"},
		{"yarn", "yarn"},
		{"svn", "svn"},
		{"node", "node"},
		{"dotnet", "dotnet"},
	}

	for _, osName := range []string{"linux", "darwin"} {
		for _, tt := range tests {
			t.Run(osName+"_"+tt.input, func(t *testing.T) {
				resolved := resolveExecutableForOS(tt.input, osName)
				if resolved != tt.expected {
					t.Errorf("for %q on %s: expected %q, got %q", tt.input, osName, tt.expected, resolved)
				}
			})
		}
	}
}
