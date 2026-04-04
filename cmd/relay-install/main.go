package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kxn/codex-remote-feishu/internal/app/install"
)

func main() {
	defaults, err := install.DetectPlatformDefaults()
	if err != nil {
		log.Fatal(err)
	}
	defaultWrapperBinary := filepath.Join(".", "bin", "codex-remote-wrapper")
	defaultRelaydBinary := filepath.Join(".", "bin", "codex-remote-relayd")
	defaultBundleEntrypoint := ""
	if defaults.GOOS == "windows" {
		defaultWrapperBinary += ".exe"
		defaultRelaydBinary += ".exe"
	}
	if len(defaults.CandidateBundleEntrypoints) > 0 {
		defaultBundleEntrypoint = defaults.CandidateBundleEntrypoints[0]
	}
	interactive := flag.Bool("interactive", false, "run interactive installer wizard")
	baseDir := flag.String("base-dir", defaults.BaseDir, "base directory for config and install state")
	installBinDir := flag.String("install-bin-dir", defaults.InstallBinDir, "target directory for installed binaries; empty keeps source paths")
	wrapperBinary := flag.String("wrapper-binary", defaultWrapperBinary, "wrapper binary source path")
	relaydBinary := flag.String("relayd-binary", defaultRelaydBinary, "relayd binary source path")
	relayURL := flag.String("relay-url", "ws://127.0.0.1:9500/ws/agent", "relay websocket url")
	codexBinary := flag.String("codex-binary", "", "real codex binary path; empty keeps wrapper default and lets managed_shim auto-resolve codex.real")
	integrationMode := flag.String("integration", "auto", "integration mode: auto, editor_settings, managed_shim, both, or comma list")
	feishuAppID := flag.String("feishu-app-id", "", "feishu app id")
	feishuSecret := flag.String("feishu-app-secret", "", "feishu app secret")
	useSystemProxy := flag.Bool("use-system-proxy", false, "whether relayd should use system proxy for Feishu API")
	settingsPath := flag.String("vscode-settings", defaults.VSCodeSettingsPath, "vscode settings path")
	bundleEntrypoint := flag.String("bundle-entrypoint", defaultBundleEntrypoint, "VS Code extension bundle codex entrypoint path")
	flag.Parse()

	integrations, err := install.ParseIntegrations(*integrationMode, defaults.GOOS)
	if err != nil {
		log.Fatal(err)
	}
	service := install.NewService()
	opts := install.Options{
		BaseDir:            *baseDir,
		InstallBinDir:      *installBinDir,
		WrapperBinary:      *wrapperBinary,
		RelaydBinary:       *relaydBinary,
		RelayServerURL:     *relayURL,
		CodexRealBinary:    *codexBinary,
		VSCodeSettingsPath: *settingsPath,
		BundleEntrypoint:   *bundleEntrypoint,
		FeishuAppID:        *feishuAppID,
		FeishuAppSecret:    *feishuSecret,
		UseSystemProxy:     *useSystemProxy,
		Integrations:       integrations,
	}
	if *interactive {
		opts, err = install.RunInteractiveWizard(os.Stdin, os.Stdout, defaults, opts)
		if err != nil {
			log.Fatal(err)
		}
	}

	state, err := service.Bootstrap(opts)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("installed wrapper config: %s\nservices config: %s\nstate: %s\nwrapper binary: %s\nrelayd binary: %s\nintegrations: %v\n", state.WrapperConfigPath, state.ServicesConfigPath, state.StatePath, state.InstalledWrapperBinary, state.InstalledRelaydBinary, state.Integrations)
}
