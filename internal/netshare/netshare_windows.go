//go:build windows

package netshare

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const resourceTypeDisk = 1

// Windows error codes returned by WNetAddConnection2.
const (
	errorAccessDenied       = 5
	errorBadNetPath         = 53
	errorBadNetName         = 67
	errorInvalidPassword    = 86
	errorSessionCredConflct = 1219
	errorNoNetOrBadPath     = 1203
	errorLogonFailure       = 1326
	errorBadUsername        = 2202
)

var (
	mpr                        = syscall.NewLazyDLL("mpr.dll")
	procWNetAddConnection2W    = mpr.NewProc("WNetAddConnection2W")
	procWNetCancelConnection2W = mpr.NewProc("WNetCancelConnection2W")
)

// Disconnect drops the connections Windows holds to the share that contains path, and the
// server's sign-in session that goes with it, so that a different account can sign in. Mapped
// drives and connections to other shares of the same server are left alone.
func Disconnect(path string) {
	root, ok := ShareRoot(path)
	if !ok {
		return
	}
	server := root[:strings.LastIndex(root, `\`)]

	for _, name := range []string{root, server + `\IPC$`} {
		ptr, err := syscall.UTF16PtrFromString(name)
		if err != nil {
			continue
		}
		// The final argument forces the disconnect even if a File Explorer window has it open.
		_, _, _ = procWNetCancelConnection2W.Call(uintptr(unsafe.Pointer(ptr)), 0, 1)
	}
}

type netResource struct {
	scope       uint32
	resType     uint32
	displayType uint32
	usage       uint32
	localName   *uint16
	remoteName  *uint16
	comment     *uint16
	provider    *uint16
}

// Connect signs in to the share that contains path with the given account. The connection is not
// mapped to a drive letter and lasts until the user logs off, like one made in File Explorer.
// The password is handed straight to Windows and never appears on a command line.
func Connect(path, username, password string) error {
	root, ok := ShareRoot(path)
	if !ok {
		return fmt.Errorf("netshare: %q is not a network folder (\\\\server\\share)", path)
	}

	remote, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return err
	}
	user, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return err
	}
	pass, err := syscall.UTF16PtrFromString(password)
	if err != nil {
		return err
	}

	resource := netResource{resType: resourceTypeDisk, remoteName: remote}
	code, _, _ := procWNetAddConnection2W.Call(
		uintptr(unsafe.Pointer(&resource)),
		uintptr(unsafe.Pointer(pass)),
		uintptr(unsafe.Pointer(user)),
		0,
	)

	switch code {
	case 0:
		return nil
	case errorLogonFailure, errorInvalidPassword, errorBadUsername:
		return ErrLogonFailed
	case errorBadNetPath, errorBadNetName, errorNoNetOrBadPath:
		return ErrNotFound
	case errorAccessDenied:
		return ErrAccessDenied
	case errorSessionCredConflct:
		return ErrConflict
	default:
		return fmt.Errorf("netshare: cannot connect to %s: %w", root, syscall.Errno(code))
	}
}
