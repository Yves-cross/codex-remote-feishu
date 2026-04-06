package daemon

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/config"
	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	relayruntime "github.com/kxn/codex-remote-feishu/internal/runtime"
)

func TestVSCodeDetectApplyAndReinstallManagedShim(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("VSCODE_SERVER_EXTENSIONS_DIR", filepath.Join(home, ".vscode-server", "extensions"))

	binaryPath := filepath.Join(home, "bin", "codex-remote")
	writeExecutableFile(t, binaryPath, "wrapper-binary")

	entrypointV1 := filepath.Join(home, ".vscode-server", "extensions", "openai.chatgpt-1", "bin", "linux-x86_64", "codex")
	writeExecutableFile(t, entrypointV1, "orig-v1")

	app, configPath, installStatePath := newVSCodeAdminTestApp(t, home, binaryPath, true)

	rec := performAdminRequest(t, app, http.MethodGet, "/api/admin/vscode/detect", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("detect status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var detect vscodeDetectResponse
	if err := json.NewDecoder(rec.Body).Decode(&detect); err != nil {
		t.Fatalf("decode detect: %v", err)
	}
	if detect.RecommendedMode != "managed_shim" {
		t.Fatalf("recommended mode = %q, want managed_shim", detect.RecommendedMode)
	}
	if detect.LatestBundleEntrypoint != entrypointV1 {
		t.Fatalf("latest bundle entrypoint = %q, want %q", detect.LatestBundleEntrypoint, entrypointV1)
	}

	rec = performAdminRequest(t, app, http.MethodPost, "/api/admin/vscode/apply", `{"mode":"managed_shim"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(entrypointV1 + ".real"); err != nil {
		t.Fatalf("expected .real backup after shim install: %v", err)
	}
	if readFileString(t, entrypointV1) != "wrapper-binary" {
		t.Fatalf("expected shimmed entrypoint to match wrapper binary")
	}

	loaded, err := config.LoadAppConfigAtPath(configPath)
	if err != nil {
		t.Fatalf("LoadAppConfigAtPath: %v", err)
	}
	if loaded.Config.Wrapper.IntegrationMode != "managed_shim" {
		t.Fatalf("wrapper integration mode = %q, want managed_shim", loaded.Config.Wrapper.IntegrationMode)
	}
	if loaded.Config.Wrapper.CodexRealBinary != "codex" {
		t.Fatalf("expected existing codexRealBinary to be preserved, got %q", loaded.Config.Wrapper.CodexRealBinary)
	}

	entrypointV2 := filepath.Join(home, ".vscode-server", "extensions", "openai.chatgpt-2", "bin", "linux-x86_64", "codex")
	writeExecutableFile(t, entrypointV2, "orig-v2")
	now := time.Now().Add(time.Minute)
	if err := os.Chtimes(filepath.Dir(filepath.Dir(filepath.Dir(entrypointV2))), now, now); err != nil {
		t.Fatalf("Chtimes(new extension dir): %v", err)
	}

	rec = performAdminRequest(t, app, http.MethodGet, "/api/admin/vscode/detect", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("detect after upgrade status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	if err := json.NewDecoder(rec.Body).Decode(&detect); err != nil {
		t.Fatalf("decode detect after upgrade: %v", err)
	}
	if detect.LatestBundleEntrypoint != entrypointV2 {
		t.Fatalf("latest bundle entrypoint after upgrade = %q, want %q", detect.LatestBundleEntrypoint, entrypointV2)
	}
	if !detect.NeedsShimReinstall {
		t.Fatalf("expected shim reinstall to be required after extension upgrade, got %#v", detect)
	}

	rec = performAdminRequest(t, app, http.MethodPost, "/api/admin/vscode/reinstall-shim", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("reinstall status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(entrypointV2 + ".real"); err != nil {
		t.Fatalf("expected .real backup on latest entrypoint: %v", err)
	}
	if readFileString(t, entrypointV2) != "wrapper-binary" {
		t.Fatalf("expected latest entrypoint to match wrapper binary")
	}
	if err := json.NewDecoder(rec.Body).Decode(&detect); err != nil {
		t.Fatalf("decode reinstall detect: %v", err)
	}
	if detect.NeedsShimReinstall {
		t.Fatalf("did not expect reinstall flag after reinstall, got %#v", detect)
	}

	rawState, err := os.ReadFile(installStatePath)
	if err != nil {
		t.Fatalf("read install-state: %v", err)
	}
	if !strings.Contains(string(rawState), entrypointV2) {
		t.Fatalf("expected install-state to record latest bundle entrypoint, got %s", string(rawState))
	}
}

func TestVSCodeApplyEditorSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binaryPath := filepath.Join(home, "bin", "codex-remote")
	writeExecutableFile(t, binaryPath, "wrapper-binary")

	app, configPath, installStatePath := newVSCodeAdminTestApp(t, home, binaryPath, false)

	rec := performAdminRequest(t, app, http.MethodPost, "/api/admin/vscode/apply", `{"mode":"editor_settings"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	settingsPath := filepath.Join(home, ".config", "Code", "User", "settings.json")
	if readFileString(t, settingsPath) == "" {
		t.Fatal("expected settings.json to be created")
	}
	loaded, err := config.LoadAppConfigAtPath(configPath)
	if err != nil {
		t.Fatalf("LoadAppConfigAtPath: %v", err)
	}
	if loaded.Config.Wrapper.IntegrationMode != "editor_settings" {
		t.Fatalf("wrapper integration mode = %q, want editor_settings", loaded.Config.Wrapper.IntegrationMode)
	}
	rawState, err := os.ReadFile(installStatePath)
	if err != nil {
		t.Fatalf("read install-state: %v", err)
	}
	if !strings.Contains(string(rawState), settingsPath) {
		t.Fatalf("expected install-state to record settings path, got %s", string(rawState))
	}

	rec = performAdminRequest(t, app, http.MethodGet, "/api/admin/vscode/detect", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("detect status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var detect vscodeDetectResponse
	if err := json.NewDecoder(rec.Body).Decode(&detect); err != nil {
		t.Fatalf("decode detect: %v", err)
	}
	if !detect.Settings.MatchesBinary {
		t.Fatalf("expected settings to point at current binary, got %#v", detect.Settings)
	}
}

func TestVSCodeDetectRecommendsAllOutsideSSH(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binaryPath := filepath.Join(home, "bin", "codex-remote")
	writeExecutableFile(t, binaryPath, "wrapper-binary")

	app, _, _ := newVSCodeAdminTestApp(t, home, binaryPath, false)

	rec := performAdminRequest(t, app, http.MethodGet, "/api/admin/vscode/detect", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("detect status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var detect vscodeDetectResponse
	if err := json.NewDecoder(rec.Body).Decode(&detect); err != nil {
		t.Fatalf("decode detect: %v", err)
	}
	if detect.RecommendedMode != "all" {
		t.Fatalf("recommended mode = %q, want all", detect.RecommendedMode)
	}
}

func TestVSCodeApplyAllAliasesBoth(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("VSCODE_SERVER_EXTENSIONS_DIR", filepath.Join(home, ".vscode-server", "extensions"))
	binaryPath := filepath.Join(home, "bin", "codex-remote")
	writeExecutableFile(t, binaryPath, "wrapper-binary")

	entrypoint := filepath.Join(home, ".vscode-server", "extensions", "openai.chatgpt-1", "bin", "linux-x86_64", "codex")
	writeExecutableFile(t, entrypoint, "orig")

	app, configPath, _ := newVSCodeAdminTestApp(t, home, binaryPath, false)

	rec := performAdminRequest(t, app, http.MethodPost, "/api/admin/vscode/apply", `{"mode":"all"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply all status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	loaded, err := config.LoadAppConfigAtPath(configPath)
	if err != nil {
		t.Fatalf("LoadAppConfigAtPath: %v", err)
	}
	if loaded.Config.Wrapper.IntegrationMode != "both" {
		t.Fatalf("wrapper integration mode = %q, want both", loaded.Config.Wrapper.IntegrationMode)
	}
}

func TestVSCodeDetectSupportsJSONCSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binaryPath := filepath.Join(home, "bin", "codex-remote")
	writeExecutableFile(t, binaryPath, "wrapper-binary")

	settingsPath := filepath.Join(home, ".config", "Code", "User", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(settings dir): %v", err)
	}
	rawSettings := "{\n  // existing vscode config\n  \"chatgpt.cliExecutable\": \"" + binaryPath + "\",\n}\n"
	if err := os.WriteFile(settingsPath, []byte(rawSettings), 0o644); err != nil {
		t.Fatalf("WriteFile(settings): %v", err)
	}

	app, _, _ := newVSCodeAdminTestApp(t, home, binaryPath, false)

	rec := performAdminRequest(t, app, http.MethodGet, "/api/admin/vscode/detect", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("detect status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var detect vscodeDetectResponse
	if err := json.NewDecoder(rec.Body).Decode(&detect); err != nil {
		t.Fatalf("decode detect: %v", err)
	}
	if detect.Settings.CLIExecutable != binaryPath {
		t.Fatalf("settings cli executable = %q, want %q", detect.Settings.CLIExecutable, binaryPath)
	}
	if !detect.Settings.MatchesBinary {
		t.Fatalf("expected settings to match current binary, got %#v", detect.Settings)
	}
}

func newVSCodeAdminTestApp(t *testing.T, home, binaryPath string, sshSession bool) (*App, string, string) {
	t.Helper()

	cfg := config.DefaultAppConfig()
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.WriteAppConfig(configPath, cfg); err != nil {
		t.Fatalf("WriteAppConfig: %v", err)
	}
	dataDir := filepath.Join(home, ".local", "share", "codex-remote")
	installStatePath := filepath.Join(dataDir, "install-state.json")

	app := New(":0", ":0", &recordingGateway{}, agentproto.ServerIdentity{})
	app.SetHeadlessRuntime(HeadlessRuntimeConfig{
		BinaryPath: binaryPath,
		Paths: relayruntime.Paths{
			DataDir:  dataDir,
			StateDir: filepath.Join(home, ".local", "state", "codex-remote"),
		},
	})
	app.ConfigureAdmin(AdminRuntimeOptions{
		ConfigPath:      configPath,
		Services:        defaultFeishuServices(),
		AdminListenHost: "127.0.0.1",
		AdminListenPort: "9501",
		AdminURL:        "http://localhost:9501/",
		SetupURL:        "http://localhost:9501/setup",
		SSHSession:      sshSession,
	})
	return app, configPath, installStatePath
}

func writeExecutableFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return string(raw)
}
