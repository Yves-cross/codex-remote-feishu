package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"fschannel/internal/app/install"
)

func main() {
	home, _ := os.UserHomeDir()
	baseDir := flag.String("base-dir", home, "base directory for config and install state")
	wrapperBinary := flag.String("wrapper-binary", "/usr/local/bin/relay-wrapper", "wrapper binary path")
	relayURL := flag.String("relay-url", "ws://127.0.0.1:9500/ws/agent", "relay websocket url")
	codexBinary := flag.String("codex-binary", "codex", "real codex binary path")
	integrationMode := flag.String("integration", string(install.IntegrationEditorSettings), "integration mode: editor_settings or managed_shim")
	feishuAppID := flag.String("feishu-app-id", "", "feishu app id")
	feishuSecret := flag.String("feishu-app-secret", "", "feishu app secret")
	settingsPath := flag.String("vscode-settings", filepath.Join(home, ".config", "Code", "User", "settings.json"), "vscode settings path")
	bundleEntrypoint := flag.String("bundle-entrypoint", "", "VS Code extension bundle codex entrypoint path")
	flag.Parse()

	mode := install.WrapperIntegrationMode(*integrationMode)
	if mode != install.IntegrationEditorSettings && mode != install.IntegrationManagedShim {
		log.Fatalf("unsupported integration mode: %s", *integrationMode)
	}

	service := install.NewService()
	state, err := service.Bootstrap(install.Options{
		BaseDir:            *baseDir,
		WrapperBinary:      *wrapperBinary,
		RelayServerURL:     *relayURL,
		CodexRealBinary:    *codexBinary,
		IntegrationMode:    mode,
		VSCodeSettingsPath: *settingsPath,
		BundleEntrypoint:   *bundleEntrypoint,
		FeishuAppID:        *feishuAppID,
		FeishuAppSecret:    *feishuSecret,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("installed wrapper config: %s\nservices config: %s\nstate: %s\n", state.WrapperConfigPath, state.ServicesConfigPath, state.StatePath)
}
