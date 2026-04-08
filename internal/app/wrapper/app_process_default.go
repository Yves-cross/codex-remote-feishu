//go:build !windows

package wrapper

import "os/exec"

func applyChildLaunchOptions(cmd *exec.Cmd, opts childLaunchOptions) {
	_ = cmd
	_ = opts
}
