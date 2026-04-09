package state

import "testing"

func TestResolveWorkspaceKey(t *testing.T) {
	if got := ResolveWorkspaceKey("", " /data/dl/work/../droid/ "); got != "/data/dl/droid" {
		t.Fatalf("ResolveWorkspaceKey() = %q, want %q", got, "/data/dl/droid")
	}
	if got := ResolveWorkspaceKey("   "); got != "" {
		t.Fatalf("ResolveWorkspaceKey() = %q, want empty", got)
	}
}

func TestWorkspaceShortName(t *testing.T) {
	if got := WorkspaceShortName("/data/dl/work/../droid/"); got != "droid" {
		t.Fatalf("WorkspaceShortName() = %q, want %q", got, "droid")
	}
	if got := WorkspaceShortName("/"); got != "/" {
		t.Fatalf("WorkspaceShortName(root) = %q, want %q", got, "/")
	}
}
