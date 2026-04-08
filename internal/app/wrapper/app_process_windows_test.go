//go:build windows

package wrapper

import (
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

func TestApplyChildLaunchOptionsWindows(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("cmd.exe")
	applyChildLaunchOptions(cmd, childLaunchOptions{
		HideWindow:     true,
		CreateNoWindow: true,
	})
	if cmd.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr to be configured")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("expected HideWindow to be enabled")
	}
	if cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatalf("expected CREATE_NO_WINDOW flag, got %#x", cmd.SysProcAttr.CreationFlags)
	}
}

func TestApplyChildLaunchOptionsWindowsPreservesUnrelatedLaunches(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("cmd.exe")
	applyChildLaunchOptions(cmd, childLaunchOptions{})
	if cmd.SysProcAttr != nil {
		t.Fatalf("expected SysProcAttr to stay nil for default launches, got %#v", cmd.SysProcAttr)
	}
}
