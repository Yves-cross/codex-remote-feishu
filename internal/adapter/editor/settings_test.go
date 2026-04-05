package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchVSCodeSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("{\"editor.fontSize\":14}\n"), 0o644); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	if err := PatchVSCodeSettings(path, "/usr/local/bin/codex-remote"); err != nil {
		t.Fatalf("patch settings: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "\"chatgpt.cliExecutable\": \"/usr/local/bin/codex-remote\"") {
		t.Fatalf("unexpected settings content: %s", text)
	}
}
