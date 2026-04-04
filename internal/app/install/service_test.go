package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/adapter/editor"
)

func TestBootstrapWritesConfigsAndState(t *testing.T) {
	baseDir := t.TempDir()
	settingsPath := filepath.Join(baseDir, "Code", "User", "settings.json")
	sourceDir := filepath.Join(baseDir, "source-bin")
	installBinDir := filepath.Join(baseDir, "installed-bin")
	wrapperBinary := seedBinary(t, filepath.Join(sourceDir, "codex-remote-wrapper"), "wrapper-bin")
	relaydBinary := seedBinary(t, filepath.Join(sourceDir, "codex-remote-relayd"), "relayd-bin")

	service := NewService()
	state, err := service.Bootstrap(Options{
		BaseDir:            baseDir,
		InstallBinDir:      installBinDir,
		WrapperBinary:      wrapperBinary,
		RelaydBinary:       relaydBinary,
		RelayServerURL:     "ws://127.0.0.1:9500/ws/agent",
		CodexRealBinary:    "/usr/local/bin/codex",
		Integrations:       []WrapperIntegrationMode{IntegrationEditorSettings},
		VSCodeSettingsPath: settingsPath,
		FeishuAppID:        "cli_xxx",
		FeishuAppSecret:    "secret",
		UseSystemProxy:     false,
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	wrapperRaw, err := os.ReadFile(state.WrapperConfigPath)
	if err != nil {
		t.Fatalf("read wrapper config: %v", err)
	}
	if !strings.Contains(string(wrapperRaw), "RELAY_SERVER_URL=ws://127.0.0.1:9500/ws/agent") {
		t.Fatalf("unexpected wrapper config: %s", wrapperRaw)
	}
	if !strings.Contains(string(wrapperRaw), "CODEX_REAL_BINARY=/usr/local/bin/codex") {
		t.Fatalf("unexpected wrapper config: %s", wrapperRaw)
	}

	serviceRaw, err := os.ReadFile(state.ServicesConfigPath)
	if err != nil {
		t.Fatalf("read services config: %v", err)
	}
	if !strings.Contains(string(serviceRaw), "FEISHU_APP_ID=cli_xxx") {
		t.Fatalf("unexpected services config: %s", serviceRaw)
	}

	settingsRaw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if !strings.Contains(string(settingsRaw), state.InstalledWrapperBinary) {
		t.Fatalf("expected settings to contain wrapper path, got %s", settingsRaw)
	}
	if state.InstalledWrapperBinary != filepath.Join(installBinDir, filepath.Base(wrapperBinary)) {
		t.Fatalf("unexpected installed wrapper path: %s", state.InstalledWrapperBinary)
	}
	if state.InstalledRelaydBinary != filepath.Join(installBinDir, filepath.Base(relaydBinary)) {
		t.Fatalf("unexpected installed relayd path: %s", state.InstalledRelaydBinary)
	}
}

func TestBootstrapManagedShimCopiesWrapperAndPreservesRealBinary(t *testing.T) {
	baseDir := t.TempDir()
	entrypoint := filepath.Join(baseDir, ".vscode-server", "extensions", "openai.chatgpt-test", "bin", "linux-x86_64", "codex")
	sourceDir := filepath.Join(baseDir, "source-bin")
	installBinDir := filepath.Join(baseDir, "installed-bin")
	wrapperBinary := seedBinary(t, filepath.Join(sourceDir, "codex-remote-wrapper"), "relay-wrapper")
	seedBinary(t, filepath.Join(sourceDir, "codex-remote-relayd"), "relayd-bin")
	seedBinary(t, entrypoint, "original-codex")

	service := NewService()
	state, err := service.Bootstrap(Options{
		BaseDir:          baseDir,
		InstallBinDir:    installBinDir,
		WrapperBinary:    wrapperBinary,
		RelaydBinary:     filepath.Join(sourceDir, "codex-remote-relayd"),
		RelayServerURL:   "ws://127.0.0.1:9500/ws/agent",
		Integrations:     []WrapperIntegrationMode{IntegrationManagedShim},
		BundleEntrypoint: entrypoint,
	})
	if err != nil {
		t.Fatalf("bootstrap managed shim: %v", err)
	}

	if state.BundleEntrypoint != entrypoint {
		t.Fatalf("unexpected bundle entrypoint in state: %s", state.BundleEntrypoint)
	}

	raw, err := os.ReadFile(entrypoint)
	if err != nil {
		t.Fatalf("read bundle entrypoint: %v", err)
	}
	if string(raw) != "relay-wrapper" {
		t.Fatalf("expected wrapper binary content in entrypoint, got %q", string(raw))
	}

	realRaw, err := os.ReadFile(editor.ManagedShimRealBinaryPath(entrypoint))
	if err != nil {
		t.Fatalf("read real binary: %v", err)
	}
	if string(realRaw) != "original-codex" {
		t.Fatalf("expected preserved real binary content, got %q", string(realRaw))
	}

	wrapperEnv, err := os.ReadFile(state.WrapperConfigPath)
	if err != nil {
		t.Fatalf("read wrapper env: %v", err)
	}
	if !strings.Contains(string(wrapperEnv), "CODEX_REAL_BINARY="+editor.ManagedShimRealBinaryPath(entrypoint)) {
		t.Fatalf("expected wrapper env to point to managed shim real binary, got %s", wrapperEnv)
	}
}

func TestBootstrapPreservesExistingFeishuSecretsWhenFlagsAreEmpty(t *testing.T) {
	baseDir := t.TempDir()
	configDir := filepath.Join(baseDir, ".config", "codex-remote")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	servicesPath := filepath.Join(configDir, "services.env")
	if err := os.WriteFile(servicesPath, []byte("RELAY_PORT=9500\nRELAY_API_PORT=9501\nFEISHU_APP_ID=cli_existing\nFEISHU_APP_SECRET=secret_existing\nFEISHU_USE_SYSTEM_PROXY=false\n"), 0o600); err != nil {
		t.Fatalf("seed services env: %v", err)
	}

	service := NewService()
	state, err := service.Bootstrap(Options{
		BaseDir:         baseDir,
		WrapperBinary:   seedBinary(t, filepath.Join(baseDir, "source-bin", "codex-remote-wrapper"), "wrapper-bin"),
		RelayServerURL:  "ws://127.0.0.1:9500/ws/agent",
		CodexRealBinary: "/usr/local/bin/codex",
		Integrations:    []WrapperIntegrationMode{IntegrationEditorSettings},
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	serviceRaw, err := os.ReadFile(state.ServicesConfigPath)
	if err != nil {
		t.Fatalf("read services config: %v", err)
	}
	text := string(serviceRaw)
	if !strings.Contains(text, "FEISHU_APP_ID=cli_existing") {
		t.Fatalf("expected app id to be preserved, got %s", text)
	}
	if !strings.Contains(text, "FEISHU_APP_SECRET=secret_existing") {
		t.Fatalf("expected app secret to be preserved, got %s", text)
	}
}

func seedBinary(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
