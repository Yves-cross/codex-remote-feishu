//go:build windows

package wrapper

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func applyChildLaunchOptions(cmd *exec.Cmd, opts childLaunchOptions) {
	if cmd == nil || (!opts.HideWindow && !opts.CreateNoWindow) {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if opts.HideWindow {
		cmd.SysProcAttr.HideWindow = true
	}
	if opts.CreateNoWindow {
		cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
	}
}
