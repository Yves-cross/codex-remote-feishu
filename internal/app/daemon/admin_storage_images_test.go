package daemon

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/config"
	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
	relayruntime "github.com/kxn/codex-remote-feishu/internal/runtime"
)

func TestImageStagingStatusAndCleanupSkipsActiveReferences(t *testing.T) {
	app, stateDir := newImageStagingAdminTestApp(t)
	rootDir := filepath.Join(stateDir, "image-staging", "app-1")

	oldFreePath := writeImageStagingFile(t, filepath.Join(rootDir, "old-free.png"), "old-free")
	oldQueuedPath := writeImageStagingFile(t, filepath.Join(rootDir, "old-queued.png"), "old-queued")
	oldStagedPath := writeImageStagingFile(t, filepath.Join(rootDir, "old-staged.png"), "old-staged")
	recentPath := writeImageStagingFile(t, filepath.Join(rootDir, "recent.png"), "recent")

	oldTime := time.Now().Add(-48 * time.Hour)
	for _, path := range []string{oldFreePath, oldQueuedPath, oldStagedPath} {
		if err := os.Chtimes(path, oldTime, oldTime); err != nil {
			t.Fatalf("Chtimes(%s): %v", path, err)
		}
	}

	app.service.UpsertInstance(&state.InstanceRecord{
		InstanceID:    "inst-1",
		DisplayName:   "Droid",
		WorkspaceRoot: t.TempDir(),
		WorkspaceKey:  t.TempDir(),
		Source:        "vscode",
		Online:        true,
		Threads:       map[string]*state.ThreadRecord{},
	})
	app.service.ApplySurfaceAction(control.Action{
		Kind:             control.ActionAttachInstance,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		InstanceID:       "inst-1",
	})
	app.service.ApplySurfaceAction(control.Action{
		Kind:             control.ActionImageMessage,
		SurfaceSessionID: "surface-1",
		MessageID:        "msg-img-queued",
		LocalPath:        oldQueuedPath,
		MIMEType:         "image/png",
	})
	app.service.ApplySurfaceAction(control.Action{
		Kind:             control.ActionTextMessage,
		SurfaceSessionID: "surface-1",
		MessageID:        "msg-text",
		Text:             "hello",
	})
	app.service.ApplySurfaceAction(control.Action{
		Kind:             control.ActionImageMessage,
		SurfaceSessionID: "surface-1",
		MessageID:        "msg-img-staged",
		LocalPath:        oldStagedPath,
		MIMEType:         "image/png",
	})

	rec := performAdminRequest(t, app, http.MethodGet, "/api/admin/storage/image-staging", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var status imageStagingStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.RootDir != filepath.Join(stateDir, "image-staging") {
		t.Fatalf("unexpected root dir: %#v", status)
	}
	if status.FileCount != 4 || status.ActiveFileCount != 2 {
		t.Fatalf("unexpected status payload: %#v", status)
	}

	rec = performAdminRequest(t, app, http.MethodPost, "/api/admin/storage/image-staging/cleanup", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("cleanup status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var cleanup imageStagingCleanupResponse
	if err := json.NewDecoder(rec.Body).Decode(&cleanup); err != nil {
		t.Fatalf("decode cleanup: %v", err)
	}
	if cleanup.OlderThanHours != defaultImageStagingCleanupHours || cleanup.DeletedFiles != 1 || cleanup.SkippedActiveCount != 2 {
		t.Fatalf("unexpected cleanup payload: %#v", cleanup)
	}
	if cleanup.RemainingFileCount != 3 {
		t.Fatalf("unexpected remaining file count: %#v", cleanup)
	}

	assertPathExists(t, oldQueuedPath)
	assertPathExists(t, oldStagedPath)
	assertPathExists(t, recentPath)
	assertPathMissing(t, oldFreePath)
}

func newImageStagingAdminTestApp(t *testing.T) (*App, string) {
	t.Helper()

	stateDir := t.TempDir()
	cfg := config.DefaultAppConfig()
	app := New(":0", ":0", &recordingGateway{}, agentproto.ServerIdentity{})
	app.SetHeadlessRuntime(HeadlessRuntimeConfig{
		Paths: relayruntime.Paths{
			StateDir: stateDir,
			LogsDir:  t.TempDir(),
		},
	})
	app.ConfigureAdmin(AdminRuntimeOptions{
		LoadConfig: func() (config.LoadedAppConfig, error) {
			return config.LoadedAppConfig{Path: "/tmp/config.json", Config: cfg}, nil
		},
		Services:        defaultFeishuServices(),
		AdminListenHost: "127.0.0.1",
		AdminListenPort: "9501",
		AdminURL:        "http://localhost:9501/",
		SetupURL:        "http://localhost:9501/setup",
	})
	return app, stateDir
}

func writeImageStagingFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
	return path
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected path to exist %s: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected path to be removed %s, err=%v", path, err)
	}
}
