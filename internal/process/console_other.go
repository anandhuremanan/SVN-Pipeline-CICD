//go:build !windows

package process

import "os/exec"

func hideConsoleWindow(cmd *exec.Cmd) {}

func killProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
