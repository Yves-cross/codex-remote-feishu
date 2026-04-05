package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/adapter/editor"
	"github.com/kxn/codex-remote-feishu/internal/config"
)

func TestBootstrapWritesConfigsAndState(t *testing.T) {
	baseDir := t.TempDir()
	settingsPath := filepath.Join(baseDir, "Code", "User", "settings.json")
	sourceDir := filepath.Join(baseDir, "source-bin")
	installBinDir := filepath.Join(baseDir, "installed-bin")
	binaryPath := seedBinary(t, filepath.Join(sourceDir, "codex-remote"), "unified-bin")

	service := NewService()
	state, err := service.Bootstrap(Options{
		BaseDir:            baseDir,
		InstallBinDir:      installBinDir,
		BinaryPath:         binaryPath,
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

	if state.ConfigPath != filepath.Join(baseDir, ".config", "codex-remote", "config.env") {
		t.Fatalf("unexpected config path: %s", state.ConfigPath)
	}
	if state.WrapperConfigPath != state.ConfigPath || state.ServicesConfigPath != state.ConfigPath {
		t.Fatalf("expected wrapper/services config paths to match unified config path")
	}

	wrapperRaw, err := os.ReadFile(state.ConfigPath)
	if err != nil {
		t.Fatalf("read wrapper config: %v", err)
	}
	if !strings.Contains(string(wrapperRaw), "RELAY_SERVER_URL=ws://127.0.0.1:9500/ws/agent") {
		t.Fatalf("unexpected wrapper config: %s", wrapperRaw)
	}
	if !strings.Contains(string(wrapperRaw), "CODEX_REAL_BINARY=/usr/local/bin/codex") {
		t.Fatalf("unexpected wrapper config: %s", wrapperRaw)
	}

	if !strings.Contains(string(wrapperRaw), "FEISHU_APP_ID=cli_xxx") {
		t.Fatalf("unexpected unified config: %s", wrapperRaw)
	}

	settingsRaw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if !strings.Contains(string(settingsRaw), state.InstalledWrapperBinary) {
		t.Fatalf("expected settings to contain wrapper path, got %s", settingsRaw)
	}
	wantBinary := filepath.Join(installBinDir, filepath.Base(binaryPath))
	if state.InstalledBinary != wantBinary {
		t.Fatalf("unexpected installed binary path: %s", state.InstalledBinary)
	}
	if state.InstalledWrapperBinary != wantBinary {
		t.Fatalf("unexpected installed wrapper alias path: %s", state.InstalledWrapperBinary)
	}
	if state.InstalledRelaydBinary != wantBinary {
		t.Fatalf("unexpected installed relayd alias path: %s", state.InstalledRelaydBinary)
	}
}

func TestBootstrapManagedShimCopiesWrapperAndPreservesRealBinary(t *testing.T) {
	baseDir := t.TempDir()
	entrypoint := filepath.Join(baseDir, ".vscode-server", "extensions", "openai.chatgpt-test", "bin", "linux-x86_64", "codex")
	sourceDir := filepath.Join(baseDir, "source-bin")
	installBinDir := filepath.Join(baseDir, "installed-bin")
	binaryPath := seedBinary(t, filepath.Join(sourceDir, "codex-remote"), "codex-remote")
	seedBinary(t, entrypoint, "original-codex")

	service := NewService()
	state, err := service.Bootstrap(Options{
		BaseDir:          baseDir,
		InstallBinDir:    installBinDir,
		BinaryPath:       binaryPath,
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
	if string(raw) != "codex-remote" {
		t.Fatalf("expected unified binary content in entrypoint, got %q", string(raw))
	}

	realRaw, err := os.ReadFile(editor.ManagedShimRealBinaryPath(entrypoint))
	if err != nil {
		t.Fatalf("read real binary: %v", err)
	}
	if string(realRaw) != "original-codex" {
		t.Fatalf("expected preserved real binary content, got %q", string(realRaw))
	}

	wrapperEnv, err := os.ReadFile(state.ConfigPath)
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
		BinaryPath:      seedBinary(t, filepath.Join(baseDir, "source-bin", "codex-remote"), "binary-bin"),
		RelayServerURL:  "ws://127.0.0.1:9500/ws/agent",
		CodexRealBinary: "/usr/local/bin/codex",
		Integrations:    []WrapperIntegrationMode{IntegrationEditorSettings},
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	serviceRaw, err := os.ReadFile(state.ConfigPath)
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
	if _, err := os.Stat(servicesPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy services.env to be removed, got err=%v", err)
	}
}

func TestBootstrapPreservesExistingDebugRelayFlowFlag(t *testing.T) {
	baseDir := t.TempDir()
	configDir := filepath.Join(baseDir, ".config", "codex-remote")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.env")
	if err := os.WriteFile(configPath, []byte(config.DebugRelayFlowEnv+"=true\n"), 0o600); err != nil {
		t.Fatalf("seed unified config: %v", err)
	}

	service := NewService()
	state, err := service.Bootstrap(Options{
		BaseDir:         baseDir,
		BinaryPath:      seedBinary(t, filepath.Join(baseDir, "source-bin", "codex-remote"), "binary-bin"),
		RelayServerURL:  "ws://127.0.0.1:9500/ws/agent",
		CodexRealBinary: "/usr/local/bin/codex",
		Integrations:    []WrapperIntegrationMode{IntegrationEditorSettings},
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	raw, err := os.ReadFile(state.ConfigPath)
	if err != nil {
		t.Fatalf("read unified config: %v", err)
	}
	if !strings.Contains(string(raw), config.DebugRelayFlowEnv+"=true") {
		t.Fatalf("expected debug relay flow flag to be preserved, got %s", raw)
	}
}

func TestBootstrapPreservesExistingDebugRelayRawFlag(t *testing.T) {
	baseDir := t.TempDir()
	configDir := filepath.Join(baseDir, ".config", "codex-remote")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.env")
	if err := os.WriteFile(configPath, []byte(config.DebugRelayRawEnv+"=true\n"), 0o600); err != nil {
		t.Fatalf("seed unified config: %v", err)
	}

	service := NewService()
	state, err := service.Bootstrap(Options{
		BaseDir:         baseDir,
		BinaryPath:      seedBinary(t, filepath.Join(baseDir, "source-bin", "codex-remote"), "binary-bin"),
		RelayServerURL:  "ws://127.0.0.1:9500/ws/agent",
		CodexRealBinary: "/usr/local/bin/codex",
		Integrations:    []WrapperIntegrationMode{IntegrationEditorSettings},
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	raw, err := os.ReadFile(state.ConfigPath)
	if err != nil {
		t.Fatalf("read unified config: %v", err)
	}
	if !strings.Contains(string(raw), config.DebugRelayRawEnv+"=true") {
		t.Fatalf("expected debug relay raw flag to be preserved, got %s", raw)
	}
}

func TestBootstrapAcceptsMatchingDeprecatedBinaryFlags(t *testing.T) {
	baseDir := t.TempDir()
	sourceBinary := seedBinary(t, filepath.Join(baseDir, "source-bin", "codex-remote"), "binary-bin")

	service := NewService()
	state, err := service.Bootstrap(Options{
		BaseDir:        baseDir,
		WrapperBinary:  sourceBinary,
		RelaydBinary:   sourceBinary,
		RelayServerURL: "ws://127.0.0.1:9500/ws/agent",
	})
	if err != nil {
		t.Fatalf("bootstrap with deprecated flags: %v", err)
	}
	if state.InstalledBinary != sourceBinary {
		t.Fatalf("InstalledBinary = %q, want %q", state.InstalledBinary, sourceBinary)
	}
}

func TestBootstrapRejectsMismatchedDeprecatedBinaryFlags(t *testing.T) {
	baseDir := t.TempDir()
	service := NewService()
	_, err := service.Bootstrap(Options{
		BaseDir:        baseDir,
		WrapperBinary:  seedBinary(t, filepath.Join(baseDir, "source-bin", "wrapper"), "wrapper"),
		RelaydBinary:   seedBinary(t, filepath.Join(baseDir, "source-bin", "daemon"), "daemon"),
		RelayServerURL: "ws://127.0.0.1:9500/ws/agent",
	})
	if err == nil || !strings.Contains(err.Error(), "single-binary install requires -binary") {
		t.Fatalf("expected mismatched deprecated binary error, got %v", err)
	}
}

func TestBootstrapMergesLegacySplitConfigFiles(t *testing.T) {
	baseDir := t.TempDir()
	configDir := filepath.Join(baseDir, ".config", "codex-remote")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	wrapperPath := filepath.Join(configDir, "wrapper.env")
	servicesPath := filepath.Join(configDir, "services.env")
	if err := os.WriteFile(wrapperPath, []byte("RELAY_SERVER_URL=ws://127.0.0.1:9910/ws/agent\nCODEX_REAL_BINARY=/legacy/codex\n"), 0o600); err != nil {
		t.Fatalf("seed wrapper env: %v", err)
	}
	if err := os.WriteFile(servicesPath, []byte("RELAY_PORT=9910\nRELAY_API_PORT=9911\nFEISHU_APP_ID=cli_old\nFEISHU_APP_SECRET=secret_old\n"), 0o600); err != nil {
		t.Fatalf("seed services env: %v", err)
	}

	service := NewService()
	state, err := service.Bootstrap(Options{
		BaseDir:         baseDir,
		BinaryPath:      seedBinary(t, filepath.Join(baseDir, "source-bin", "codex-remote"), "binary-bin"),
		RelayServerURL:  "ws://127.0.0.1:9500/ws/agent",
		CodexRealBinary: "/usr/local/bin/codex",
		Integrations:    []WrapperIntegrationMode{IntegrationEditorSettings},
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	raw, err := os.ReadFile(state.ConfigPath)
	if err != nil {
		t.Fatalf("read unified config: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "FEISHU_APP_ID=cli_old") || !strings.Contains(text, "FEISHU_APP_SECRET=secret_old") {
		t.Fatalf("expected legacy services values in unified config, got %s", text)
	}
	if _, err := os.Stat(wrapperPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy wrapper.env to be removed, got err=%v", err)
	}
	if _, err := os.Stat(servicesPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy services.env to be removed, got err=%v", err)
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
