package orchestrator

import "testing"

func TestWorkspaceCreatePickerRootForGOOSWindowsUsesInitialPathVolume(t *testing.T) {
	got := workspaceCreatePickerRootForGOOS("windows", `E:\temp\demo`)
	if got != "E:/" {
		t.Fatalf("workspaceCreatePickerRootForGOOS(windows) = %q, want %q", got, "E:/")
	}
}

func TestWorkspaceCreatePickerRootForGOOSUnixUsesFilesystemRoot(t *testing.T) {
	got := workspaceCreatePickerRootForGOOS("linux", "/tmp/demo")
	if got != "/" {
		t.Fatalf("workspaceCreatePickerRootForGOOS(linux) = %q, want /", got)
	}
}
