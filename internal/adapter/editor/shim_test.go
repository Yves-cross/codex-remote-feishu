package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchBundleEntrypoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin", "linux-x86_64", "codex")
	if err := PatchBundleEntrypoint(path, "/usr/local/bin/codex-remote-wrapper"); err != nil {
		t.Fatalf("patch bundle entrypoint: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read bundle entrypoint: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "WRAPPER_BIN=\"/usr/local/bin/codex-remote-wrapper\"") {
		t.Fatalf("unexpected bundle content: %s", text)
	}
	if !strings.Contains(text, "export CODEX_REAL_BINARY=\"${CODEX_REAL_BINARY:-$REAL_BIN}\"") {
		t.Fatalf("missing real binary export: %s", text)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat bundle entrypoint: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("unexpected mode: %o", info.Mode().Perm())
	}
}
