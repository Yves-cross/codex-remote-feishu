package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/adapter/feishu"
	"github.com/kxn/codex-remote-feishu/internal/config"
	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	relayruntime "github.com/kxn/codex-remote-feishu/internal/runtime"
)

type fakePreviewDriveAdmin struct {
	summary         feishu.PreviewDriveSummary
	cleanupResult   feishu.PreviewDriveCleanupResult
	reconcileResult feishu.PreviewDriveReconcileResult
	summaryErr      error
	cleanupErr      error
	reconcileErr    error
	cleanupCutoff   time.Time
}

func (f *fakePreviewDriveAdmin) Summary() (feishu.PreviewDriveSummary, error) {
	return f.summary, f.summaryErr
}

func (f *fakePreviewDriveAdmin) CleanupBefore(_ context.Context, cutoff time.Time) (feishu.PreviewDriveCleanupResult, error) {
	f.cleanupCutoff = cutoff
	return f.cleanupResult, f.cleanupErr
}

func (f *fakePreviewDriveAdmin) Reconcile(context.Context) (feishu.PreviewDriveReconcileResult, error) {
	return f.reconcileResult, f.reconcileErr
}

func TestPreviewDriveStatusAndCleanupRoutes(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Feishu.Apps = []config.FeishuAppConfig{{
		ID:        "main",
		Name:      "Main Bot",
		AppID:     "cli_xxx",
		AppSecret: "secret_xxx",
	}}
	app, stateDir := newPreviewDriveAdminTestApp(t, cfg)

	fake := &fakePreviewDriveAdmin{
		summary: feishu.PreviewDriveSummary{
			StatePath:            filepath.Join(stateDir, "feishu-md-preview-main.json"),
			RootToken:            "fld-root",
			RootURL:              "https://preview/root",
			FileCount:            2,
			ScopeCount:           1,
			EstimatedBytes:       1234,
			UnknownSizeFileCount: 1,
		},
		cleanupResult: feishu.PreviewDriveCleanupResult{
			DeletedFileCount:            1,
			DeletedEstimatedBytes:       120,
			SkippedUnknownLastUsedCount: 1,
			Summary: feishu.PreviewDriveSummary{
				FileCount:            1,
				ScopeCount:           1,
				EstimatedBytes:       1114,
				UnknownSizeFileCount: 0,
			},
		},
		reconcileResult: feishu.PreviewDriveReconcileResult{
			Summary: feishu.PreviewDriveSummary{
				FileCount:      1,
				ScopeCount:     1,
				EstimatedBytes: 1114,
			},
			RemoteMissingScopeCount: 1,
			RemoteMissingFileCount:  2,
			LocalOnlyFileCount:      1,
			PermissionDriftCount:    1,
		},
	}

	originalFactory := newPreviewDriveAdminService
	defer func() { newPreviewDriveAdminService = originalFactory }()

	var capturedCfg feishu.GatewayAppConfig
	newPreviewDriveAdminService = func(cfg feishu.GatewayAppConfig) feishu.PreviewDriveAdminService {
		capturedCfg = cfg
		return fake
	}

	rec := performAdminRequest(t, app, http.MethodGet, "/api/admin/storage/preview-drive/main", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var status previewDriveStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.GatewayID != "main" || status.Summary.FileCount != 2 {
		t.Fatalf("unexpected status payload: %#v", status)
	}
	if capturedCfg.GatewayID != "main" || capturedCfg.PreviewStatePath != filepath.Join(stateDir, "feishu-md-preview-main.json") {
		t.Fatalf("unexpected runtime preview config: %#v", capturedCfg)
	}

	before := time.Now()
	rec = performAdminRequest(t, app, http.MethodPost, "/api/admin/storage/preview-drive/main/cleanup", "")
	after := time.Now()
	if rec.Code != http.StatusOK {
		t.Fatalf("cleanup status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var cleanup previewDriveCleanupResponse
	if err := json.NewDecoder(rec.Body).Decode(&cleanup); err != nil {
		t.Fatalf("decode cleanup: %v", err)
	}
	if cleanup.GatewayID != "main" || cleanup.OlderThanHours != defaultImageStagingCleanupHours || cleanup.Result.DeletedFileCount != 1 {
		t.Fatalf("unexpected cleanup payload: %#v", cleanup)
	}
	minCutoff := before.Add(-time.Duration(defaultImageStagingCleanupHours) * time.Hour).Add(-2 * time.Second)
	maxCutoff := after.Add(-time.Duration(defaultImageStagingCleanupHours) * time.Hour).Add(2 * time.Second)
	if fake.cleanupCutoff.Before(minCutoff) || fake.cleanupCutoff.After(maxCutoff) {
		t.Fatalf("unexpected cleanup cutoff: %s not in [%s, %s]", fake.cleanupCutoff, minCutoff, maxCutoff)
	}

	rec = performAdminRequest(t, app, http.MethodPost, "/api/admin/storage/preview-drive/main/reconcile", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("reconcile status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var reconcile previewDriveReconcileResponse
	if err := json.NewDecoder(rec.Body).Decode(&reconcile); err != nil {
		t.Fatalf("decode reconcile: %v", err)
	}
	if reconcile.GatewayID != "main" || reconcile.Result.RemoteMissingScopeCount != 1 || reconcile.Result.PermissionDriftCount != 1 {
		t.Fatalf("unexpected reconcile payload: %#v", reconcile)
	}
}

func TestPreviewDriveCleanupReturnsConflictWithoutAPI(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Feishu.Apps = []config.FeishuAppConfig{{
		ID:   "main",
		Name: "Main Bot",
	}}
	app, _ := newPreviewDriveAdminTestApp(t, cfg)

	rec := performAdminRequest(t, app, http.MethodPost, "/api/admin/storage/preview-drive/main/cleanup", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("cleanup status = %d, want 409 body=%s", rec.Code, rec.Body.String())
	}

	rec = performAdminRequest(t, app, http.MethodPost, "/api/admin/storage/preview-drive/main/reconcile", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("reconcile status = %d, want 409 body=%s", rec.Code, rec.Body.String())
	}
}

func newPreviewDriveAdminTestApp(t *testing.T, cfg config.AppConfig) (*App, string) {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.WriteAppConfig(configPath, cfg); err != nil {
		t.Fatalf("WriteAppConfig: %v", err)
	}

	stateDir := t.TempDir()
	app := New(":0", ":0", &recordingGateway{}, agentproto.ServerIdentity{})
	app.SetHeadlessRuntime(HeadlessRuntimeConfig{
		Paths: relayruntime.Paths{
			StateDir: stateDir,
			LogsDir:  t.TempDir(),
		},
	})
	app.ConfigureAdmin(AdminRuntimeOptions{
		ConfigPath:      configPath,
		Services:        defaultFeishuServices(),
		AdminListenHost: "127.0.0.1",
		AdminListenPort: "9501",
		AdminURL:        "http://localhost:9501/",
		SetupURL:        "http://localhost:9501/setup",
	})
	return app, stateDir
}
