//go:build !windows

package main

func waitForProcessExit(pid int) {}

func fixedDriveRoots() []string {
	return nil
}

func diskSpace(path string) (free, total uint64, ok bool) {
	return 0, 0, false
}
