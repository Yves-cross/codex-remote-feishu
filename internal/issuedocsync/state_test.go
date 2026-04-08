package issuedocsync

import (
	"path/filepath"
	"testing"
)

func TestLoadStateMissingFileReturnsEmptyTrackedShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	state, err := LoadState(path, "kxn/codex-remote-feishu")
	if err != nil {
		t.Fatalf("LoadState missing file error = %v", err)
	}
	if state.Version != 1 || state.Repo != "kxn/codex-remote-feishu" {
		t.Fatalf("unexpected default state: %#v", state)
	}
	if len(state.Issues) != 0 {
		t.Fatalf("expected empty issue map, got %#v", state.Issues)
	}
}
