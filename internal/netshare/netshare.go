// Package netshare signs in to Windows network shares (\\server\share) that need an account
// different from the one the current user is logged on with.
package netshare

import (
	"errors"
	"strings"
)

var (
	// ErrLogonFailed means the server rejected the user name or password.
	ErrLogonFailed = errors.New("netshare: the user name or password is incorrect")
	// ErrNotFound means the server or the share could not be found.
	ErrNotFound = errors.New("netshare: the network folder could not be found")
	// ErrAccessDenied means the account is valid but may not use the share.
	ErrAccessDenied = errors.New("netshare: access to the network folder is denied")
	// ErrConflict means Windows is already connected to the server with a different account.
	ErrConflict = errors.New("netshare: already connected to this server with a different account")
	// ErrUnsupported means network share sign-in is not available on this operating system.
	ErrUnsupported = errors.New("netshare: network share sign-in is only supported on Windows")
)

// ShareRoot returns the \\server\share part of a UNC path and whether path is a UNC path at all.
func ShareRoot(path string) (string, bool) {
	path = strings.ReplaceAll(strings.TrimSpace(path), "/", `\`)
	if !strings.HasPrefix(path, `\\`) {
		return "", false
	}

	parts := strings.Split(strings.TrimPrefix(path, `\\`), `\`)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	return `\\` + parts[0] + `\` + parts[1], true
}
