//go:build windows

package process

import (
	"os/exec"
	"strconv"
	"syscall"
)

const createNoWindow = 0x08000000

var procGetConsoleWindow = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow")

// hideConsoleWindow stops child tools (svn, npm, dotnet) from flashing a console window
// when the engine runs inside a GUI process. Console runs are left untouched.
func hideConsoleWindow(cmd *exec.Cmd) {
	if hwnd, _, _ := procGetConsoleWindow.Call(); hwnd != 0 {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}

// killProcessTree stops cmd and everything it started. Tools such as npm run through a .cmd
// wrapper, so killing only the wrapper would leave the real node process running.
func killProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	if err := kill.Run(); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
