package state

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestResolveWorkspaceRootOnHostResolvesSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	resolved, err := ResolveWorkspaceRootOnHost(filepath.Join(link, ".", ""))
	if err != nil {
		t.Fatalf("ResolveWorkspaceRootOnHost() error = %v", err)
	}
	if resolved != target {
		t.Fatalf("ResolveWorkspaceRootOnHost() = %q, want %q", resolved, target)
	}
}
