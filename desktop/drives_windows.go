//go:build windows

package main

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

const driveFixed = 3

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procGetLogicalDrives    = kernel32.NewProc("GetLogicalDrives")
	procGetDriveTypeW       = kernel32.NewProc("GetDriveTypeW")
	procGetDiskFreeSpaceExW = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// fixedDriveRoots lists the roots of the PC's internal drives, e.g. "C:\", "D:\".
func fixedDriveRoots() []string {
	mask, _, _ := procGetLogicalDrives.Call()

	var roots []string
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		ptr, err := syscall.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		if kind, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(ptr))); kind == driveFixed {
			roots = append(roots, root)
		}
	}
	return roots
}

// diskSpace returns the free and total bytes of the drive that holds path. The path does not
// have to exist yet; its closest existing parent is measured.
func diskSpace(path string) (free, total uint64, ok bool) {
	for {
		ptr, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			return 0, 0, false
		}
		ret, _, _ := procGetDiskFreeSpaceExW.Call(
			uintptr(unsafe.Pointer(ptr)),
			uintptr(unsafe.Pointer(&free)),
			uintptr(unsafe.Pointer(&total)),
			0,
		)
		if ret != 0 {
			return free, total, true
		}

		parent := filepath.Dir(path)
		if parent == path {
			return 0, 0, false
		}
		path = parent
	}
}
