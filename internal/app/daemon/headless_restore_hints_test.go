package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHeadlessRestoreHintStoreRoundTrip(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	path := headlessRestoreHintsStatePath(stateDir)
	store, err := loadHeadlessRestoreHintStore(path)
	if err != nil {
		t.Fatalf("load empty store: %v", err)
	}

	updatedAt := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	if err := store.Put(HeadlessRestoreHint{
		SurfaceSessionID: "feishu:app-1:chat:chat-1",
		GatewayID:        "app-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		ThreadID:         "thread-1",
		ThreadTitle:      "修复登录流程",
		ThreadCWD:        "/data/dl/droid",
		UpdatedAt:        updatedAt,
	}); err != nil {
		t.Fatalf("put hint: %v", err)
	}

	reloaded, err := loadHeadlessRestoreHintStore(path)
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	hint, ok := reloaded.Get("feishu:app-1:chat:chat-1")
	if !ok {
		t.Fatal("expected restore hint after reload")
	}
	if hint.GatewayID != "app-1" || hint.ChatID != "chat-1" || hint.ActorUserID != "user-1" {
		t.Fatalf("unexpected restored routing fields: %#v", hint)
	}
	if hint.ThreadID != "thread-1" || hint.ThreadTitle != "修复登录流程" || hint.ThreadCWD != "/data/dl/droid" {
		t.Fatalf("unexpected restored thread fields: %#v", hint)
	}
	if !hint.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("unexpected updatedAt: %s", hint.UpdatedAt)
	}
}

func TestHeadlessRestoreHintStoreDeletePersists(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	path := headlessRestoreHintsStatePath(stateDir)
	store, err := loadHeadlessRestoreHintStore(path)
	if err != nil {
		t.Fatalf("load empty store: %v", err)
	}
	if err := store.Put(HeadlessRestoreHint{
		SurfaceSessionID: "surface-1",
		ThreadID:         "thread-1",
		UpdatedAt:        time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("put hint: %v", err)
	}
	if err := store.Delete("surface-1"); err != nil {
		t.Fatalf("delete hint: %v", err)
	}

	reloaded, err := loadHeadlessRestoreHintStore(path)
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	if _, ok := reloaded.Get("surface-1"); ok {
		t.Fatalf("expected restore hint to be deleted, got %#v", reloaded.Entries())
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read store file: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("expected non-empty store file")
	}
}

func TestHeadlessRestoreHintsStatePath(t *testing.T) {
	t.Parallel()

	stateDir := filepath.Join("/tmp", "codex-remote-state")
	if got := headlessRestoreHintsStatePath(stateDir); got != filepath.Join(stateDir, headlessRestoreHintsStateFile) {
		t.Fatalf("unexpected state path: %s", got)
	}
	if got := headlessRestoreHintsStatePath(""); got != "" {
		t.Fatalf("expected empty state path, got %q", got)
	}
}
