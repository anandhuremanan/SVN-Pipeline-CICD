package process

import (
	"runtime"
	"strings"
)

// windowsCmdMap maps CLI commands to their Windows batch script equivalents.
var windowsCmdMap = map[string]string{
	"npm":  "npm.cmd",
	"npx":  "npx.cmd",
	"pnpm": "pnpm.cmd",
	"yarn": "yarn.cmd",
}

// ResolveExecutable maps CLI commands to platform-aware executable binary names.
// On Windows, Node package managers (npm, npx, pnpm, yarn) are mapped to their .cmd extension variants.
// On Linux/macOS, the command is returned unchanged.
func ResolveExecutable(command string) string {
	if command == "" {
		return ""
	}

	return resolveExecutableForOS(command, runtime.GOOS)
}

func resolveExecutableForOS(command, goos string) string {
	if command == "" {
		return ""
	}

	if goos == "windows" {
		lower := strings.ToLower(command)
		if resolved, ok := windowsCmdMap[lower]; ok {
			return resolved
		}
	}

	return command
}
