package editor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPatchBundleEntrypoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin", "linux-x86_64", "codex")
	wrapper := filepath.Join(dir, "wrapper", "codex-remote-wrapper")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir entrypoint dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(wrapper), 0o755); err != nil {
		t.Fatalf("mkdir wrapper dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("original-codex"), 0o755); err != nil {
		t.Fatalf("seed bundle entrypoint: %v", err)
	}
	if err := os.WriteFile(wrapper, []byte("relay-wrapper"), 0o755); err != nil {
		t.Fatalf("seed wrapper binary: %v", err)
	}
	if err := PatchBundleEntrypoint(path, wrapper); err != nil {
		t.Fatalf("patch bundle entrypoint: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read bundle entrypoint: %v", err)
	}
	if string(raw) != "relay-wrapper" {
		t.Fatalf("expected copied wrapper binary, got %q", string(raw))
	}

	realPath := ManagedShimRealBinaryPath(path)
	realRaw, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("read preserved real binary: %v", err)
	}
	if string(realRaw) != "original-codex" {
		t.Fatalf("expected preserved real binary content, got %q", string(realRaw))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat bundle entrypoint: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("unexpected mode: %o", info.Mode().Perm())
	}
}

func TestManagedShimRealBinaryPath(t *testing.T) {
	tests := map[string]string{
		"/tmp/codex":     "/tmp/codex.real",
		`C:\tmp\codex`:   `C:\tmp\codex.real`,
		"/tmp/codex.exe": "/tmp/codex.real.exe",
	}
	for input, want := range tests {
		if got := ManagedShimRealBinaryPath(input); got != want {
			t.Fatalf("ManagedShimRealBinaryPath(%q) = %q, want %q", input, got, want)
		}
	}
}
