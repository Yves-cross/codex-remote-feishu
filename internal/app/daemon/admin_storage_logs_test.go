package daemon

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/adapter/feishu"
	"github.com/kxn/codex-remote-feishu/internal/config"
	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	relayruntime "github.com/kxn/codex-remote-feishu/internal/runtime"
)

func TestLogsStorageStatusAndCleanup(t *testing.T) {
	app, logsDir := newLogsStorageAdminTestApp(t)

	oldRelay := writeImageStagingFile(t, filepath.Join(logsDir, "relayd", "relay-old.log"), "relay-old")
	oldWrapper := writeImageStagingFile(t, filepath.Join(logsDir, "wrapper", "wrapper-old.log"), "wrapper-old")
	recent := writeImageStagingFile(t, filepath.Join(logsDir, "wrapper", "wrapper-recent.log"), "wrapper-recent")

	oldTime := time.Now().Add(-48 * time.Hour)
	for _, path := range []string{oldRelay, oldWrapper} {
		if err := os.Chtimes(path, oldTime, oldTime); err != nil {
			t.Fatalf("Chtimes(%s): %v", path, err)
		}
	}

	rec := performAdminRequest(t, app, http.MethodGet, "/api/admin/storage/logs", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("logs status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var status logsStorageStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode logs status: %v", err)
	}
	if status.RootDir != logsDir || status.FileCount != 3 || status.TotalBytes == 0 || status.LatestFileAt == nil {
		t.Fatalf("unexpected logs status payload: %#v", status)
	}

	rec = performAdminRequest(t, app, http.MethodPost, "/api/admin/storage/logs/cleanup", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("logs cleanup = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var cleanup logsStorageCleanupResponse
	if err := json.NewDecoder(rec.Body).Decode(&cleanup); err != nil {
		t.Fatalf("decode logs cleanup: %v", err)
	}
	if cleanup.DeletedFiles != 2 || cleanup.RemainingFileCount != 1 || cleanup.RemainingBytes == 0 {
		t.Fatalf("unexpected logs cleanup payload: %#v", cleanup)
	}

	assertPathMissing(t, oldRelay)
	assertPathMissing(t, oldWrapper)
	assertPathExists(t, recent)
}

func newLogsStorageAdminTestApp(t *testing.T) (*App, string) {
	t.Helper()

	logsDir := t.TempDir()
	cfg := config.DefaultAppConfig()
	app := New(":0", ":0", feishu.NopGateway{}, agentproto.ServerIdentity{})
	app.SetHeadlessRuntime(HeadlessRuntimeConfig{
		Paths: relayruntime.Paths{
			StateDir: t.TempDir(),
			LogsDir:  logsDir,
		},
	})
	app.ConfigureAdmin(AdminRuntimeOptions{
		LoadConfig: func() (config.LoadedAppConfig, error) {
			return config.LoadedAppConfig{Path: "/tmp/config.json", Config: cfg}, nil
		},
		Services:        defaultFeishuServices(),
		AdminListenHost: "127.0.0.1",
		AdminListenPort: "9501",
		AdminURL:        "http://localhost:9501/admin/",
		SetupURL:        "http://localhost:9501/setup",
	})
	return app, logsDir
}
