package config

import (
	"path/filepath"
	"testing"
)

func TestLoadWrapperConfigUsesUnifiedDefaultFile(t *testing.T) {
	xdgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgHome)

	configPath := filepath.Join(xdgHome, "codex-remote", "config.env")
	if err := WriteEnvFile(configPath, map[string]string{
		"RELAY_SERVER_URL":               "ws://127.0.0.1:9600/ws/agent",
		"CODEX_REAL_BINARY":              "/opt/codex",
		"CODEX_REMOTE_WRAPPER_NAME_MODE": "workspace_basename",
		DebugRelayFlowEnv:                "true",
		DebugRelayRawEnv:                 "true",
	}); err != nil {
		t.Fatalf("write unified env: %v", err)
	}

	cfg, err := LoadWrapperConfig()
	if err != nil {
		t.Fatalf("LoadWrapperConfig: %v", err)
	}
	if cfg.ConfigPath != configPath {
		t.Fatalf("ConfigPath = %q, want %q", cfg.ConfigPath, configPath)
	}
	if cfg.RelayServerURL != "ws://127.0.0.1:9600/ws/agent" {
		t.Fatalf("RelayServerURL = %q", cfg.RelayServerURL)
	}
	if cfg.CodexRealBinary != "/opt/codex" {
		t.Fatalf("CodexRealBinary = %q", cfg.CodexRealBinary)
	}
	if !cfg.DebugRelayFlow {
		t.Fatal("expected DebugRelayFlow to be true")
	}
	if !cfg.DebugRelayRaw {
		t.Fatal("expected DebugRelayRaw to be true")
	}
}

func TestLoadServicesConfigUsesUnifiedConfigEnvOverride(t *testing.T) {
	xdgHome := t.TempDir()
	overrideDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgHome)

	overridePath := filepath.Join(overrideDir, "custom.env")
	if err := WriteEnvFile(overridePath, map[string]string{
		"RELAY_PORT":              "9700",
		"RELAY_API_PORT":          "9701",
		"FEISHU_APP_ID":           "cli_override",
		"FEISHU_APP_SECRET":       "secret_override",
		"FEISHU_USE_SYSTEM_PROXY": "true",
		DebugRelayFlowEnv:         "true",
		DebugRelayRawEnv:          "true",
	}); err != nil {
		t.Fatalf("write override env: %v", err)
	}
	t.Setenv(UnifiedConfigEnvPath, overridePath)

	cfg, err := LoadServicesConfig()
	if err != nil {
		t.Fatalf("LoadServicesConfig: %v", err)
	}
	if cfg.ConfigPath != overridePath {
		t.Fatalf("ConfigPath = %q, want %q", cfg.ConfigPath, overridePath)
	}
	if cfg.RelayPort != "9700" || cfg.RelayAPIPort != "9701" {
		t.Fatalf("ports = %q/%q", cfg.RelayPort, cfg.RelayAPIPort)
	}
	if cfg.FeishuAppID != "cli_override" || cfg.FeishuAppSecret != "secret_override" {
		t.Fatalf("feishu = %q/%q", cfg.FeishuAppID, cfg.FeishuAppSecret)
	}
	if !cfg.FeishuUseSystemProxy {
		t.Fatal("expected FeishuUseSystemProxy to be true")
	}
	if !cfg.DebugRelayFlow {
		t.Fatal("expected DebugRelayFlow to be true")
	}
	if !cfg.DebugRelayRaw {
		t.Fatal("expected DebugRelayRaw to be true")
	}
}

func TestLoadersFallbackToLegacySplitFiles(t *testing.T) {
	xdgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgHome)

	wrapperPath := filepath.Join(xdgHome, "codex-remote", "wrapper.env")
	servicesPath := filepath.Join(xdgHome, "codex-remote", "services.env")
	if err := WriteEnvFile(wrapperPath, map[string]string{
		"RELAY_SERVER_URL":  "ws://127.0.0.1:9800/ws/agent",
		"CODEX_REAL_BINARY": "/legacy/codex",
	}); err != nil {
		t.Fatalf("write wrapper legacy env: %v", err)
	}
	if err := WriteEnvFile(servicesPath, map[string]string{
		"RELAY_PORT":        "9800",
		"RELAY_API_PORT":    "9801",
		"FEISHU_APP_ID":     "cli_legacy",
		"FEISHU_APP_SECRET": "secret_legacy",
	}); err != nil {
		t.Fatalf("write services legacy env: %v", err)
	}

	wrapperCfg, err := LoadWrapperConfig()
	if err != nil {
		t.Fatalf("LoadWrapperConfig: %v", err)
	}
	if wrapperCfg.ConfigPath != wrapperPath {
		t.Fatalf("wrapper ConfigPath = %q, want %q", wrapperCfg.ConfigPath, wrapperPath)
	}
	if wrapperCfg.CodexRealBinary != "/legacy/codex" {
		t.Fatalf("wrapper CodexRealBinary = %q", wrapperCfg.CodexRealBinary)
	}

	servicesCfg, err := LoadServicesConfig()
	if err != nil {
		t.Fatalf("LoadServicesConfig: %v", err)
	}
	if servicesCfg.ConfigPath != servicesPath {
		t.Fatalf("services ConfigPath = %q, want %q", servicesCfg.ConfigPath, servicesPath)
	}
	if servicesCfg.FeishuAppID != "cli_legacy" {
		t.Fatalf("services FeishuAppID = %q", servicesCfg.FeishuAppID)
	}
}
