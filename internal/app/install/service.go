package install

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/adapter/editor"
	"github.com/kxn/codex-remote-feishu/internal/config"
)

type Options struct {
	BaseDir            string
	InstallBinDir      string
	BinaryPath         string
	WrapperBinary      string
	RelaydBinary       string
	RelayServerURL     string
	CodexRealBinary    string
	IntegrationMode    WrapperIntegrationMode
	Integrations       []WrapperIntegrationMode
	VSCodeSettingsPath string
	BundleEntrypoint   string
	FeishuAppID        string
	FeishuAppSecret    string
	UseSystemProxy     bool
}

type InstallState struct {
	ConfigPath             string                   `json:"configPath,omitempty"`
	WrapperConfigPath      string                   `json:"wrapperConfigPath"`
	ServicesConfigPath     string                   `json:"servicesConfigPath"`
	StatePath              string                   `json:"statePath"`
	InstalledBinary        string                   `json:"installedBinary,omitempty"`
	InstalledWrapperBinary string                   `json:"installedWrapperBinary,omitempty"`
	InstalledRelaydBinary  string                   `json:"installedRelaydBinary,omitempty"`
	Integrations           []WrapperIntegrationMode `json:"integrations"`
	VSCodeSettingsPath     string                   `json:"vscodeSettingsPath,omitempty"`
	BundleEntrypoint       string                   `json:"bundleEntrypoint,omitempty"`
}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Bootstrap(opts Options) (InstallState, error) {
	configDir := filepath.Join(opts.BaseDir, ".config", "codex-remote")
	stateDir := filepath.Join(opts.BaseDir, ".local", "share", "codex-remote")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return InstallState{}, err
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return InstallState{}, err
	}
	sourceBinary, err := resolveBinaryPath(opts)
	if err != nil {
		return InstallState{}, err
	}
	installedBinary, err := installBinary(sourceBinary, opts.InstallBinDir)
	if err != nil {
		return InstallState{}, err
	}

	integrations := opts.Integrations
	if len(integrations) == 0 && opts.IntegrationMode != "" {
		integrations = []WrapperIntegrationMode{opts.IntegrationMode}
	}
	integrations = normalizeIntegrations(integrations)

	configPath := filepath.Join(configDir, "config.env")
	legacyWrapperConfigPath := filepath.Join(configDir, "wrapper.env")
	legacyServicesConfigPath := filepath.Join(configDir, "services.env")
	statePath := filepath.Join(stateDir, "install-state.json")
	existingValues, err := loadExistingConfigValues(configPath, legacyWrapperConfigPath, legacyServicesConfigPath)
	if err != nil {
		return InstallState{}, err
	}

	codexRealBinary := opts.CodexRealBinary
	if codexRealBinary == "" && hasIntegration(integrations, IntegrationManagedShim) && opts.BundleEntrypoint != "" {
		codexRealBinary = editor.ManagedShimRealBinaryPath(opts.BundleEntrypoint)
	}
	if installedBinary == "" {
		installedBinary = sourceBinary
	}

	if err := config.WriteEnvFile(configPath, map[string]string{
		"RELAY_SERVER_URL":                      opts.RelayServerURL,
		"CODEX_REAL_BINARY":                     codexRealBinary,
		"CODEX_REMOTE_WRAPPER_NAME_MODE":        "workspace_basename",
		"CODEX_REMOTE_WRAPPER_INTEGRATION_MODE": integrationsConfigValue(integrations),
		"RELAY_PORT":                            "9500",
		"RELAY_API_PORT":                        "9501",
		"FEISHU_APP_ID":                         choosePreservedValue(opts.FeishuAppID, existingValues["FEISHU_APP_ID"]),
		"FEISHU_APP_SECRET":                     choosePreservedValue(opts.FeishuAppSecret, existingValues["FEISHU_APP_SECRET"]),
		"FEISHU_USE_SYSTEM_PROXY":               boolString(opts.UseSystemProxy),
	}); err != nil {
		return InstallState{}, err
	}
	cleanupLegacyConfigPath(legacyWrapperConfigPath, configPath)
	cleanupLegacyConfigPath(legacyServicesConfigPath, configPath)

	if hasIntegration(integrations, IntegrationEditorSettings) && opts.VSCodeSettingsPath != "" {
		if err := editor.PatchVSCodeSettings(opts.VSCodeSettingsPath, installedBinary); err != nil {
			return InstallState{}, err
		}
	}
	if hasIntegration(integrations, IntegrationManagedShim) {
		if err := editor.PatchBundleEntrypoint(opts.BundleEntrypoint, installedBinary); err != nil {
			return InstallState{}, err
		}
	}

	state := InstallState{
		ConfigPath:             configPath,
		WrapperConfigPath:      configPath,
		ServicesConfigPath:     configPath,
		StatePath:              statePath,
		InstalledBinary:        installedBinary,
		InstalledWrapperBinary: installedBinary,
		InstalledRelaydBinary:  installedBinary,
		Integrations:           integrations,
		VSCodeSettingsPath:     opts.VSCodeSettingsPath,
		BundleEntrypoint:       opts.BundleEntrypoint,
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return InstallState{}, err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(statePath, raw, 0o644); err != nil {
		return InstallState{}, err
	}

	return state, nil
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func choosePreservedValue(incoming, existing string) string {
	if strings.TrimSpace(incoming) != "" {
		return incoming
	}
	return existing
}

func installBinary(sourcePath, installDir string) (string, error) {
	if strings.TrimSpace(sourcePath) == "" {
		return "", nil
	}
	if strings.TrimSpace(installDir) == "" {
		return sourcePath, nil
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return "", err
	}
	targetPath := filepath.Join(installDir, filepath.Base(sourcePath))
	if samePath(sourcePath, targetPath) {
		return targetPath, nil
	}
	if err := copyFile(sourcePath, targetPath); err != nil {
		return "", err
	}
	return targetPath, nil
}

func samePath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

func copyFile(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	info, err := sourceFile.Stat()
	if err != nil {
		return err
	}
	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return err
	}
	return targetFile.Chmod(info.Mode().Perm())
}

func resolveBinaryPath(opts Options) (string, error) {
	binaryPath := strings.TrimSpace(opts.BinaryPath)
	wrapperBinary := strings.TrimSpace(opts.WrapperBinary)
	relaydBinary := strings.TrimSpace(opts.RelaydBinary)

	if binaryPath != "" {
		return binaryPath, nil
	}
	if wrapperBinary != "" && relaydBinary != "" && !samePath(wrapperBinary, relaydBinary) {
		return "", fmt.Errorf("single-binary install requires -binary, or matching deprecated -wrapper-binary and -relayd-binary")
	}
	if wrapperBinary != "" {
		return wrapperBinary, nil
	}
	if relaydBinary != "" {
		return relaydBinary, nil
	}
	return "", fmt.Errorf("binary path is required")
}

func loadExistingConfigValues(configPath, legacyWrapperConfigPath, legacyServicesConfigPath string) (map[string]string, error) {
	if values, err := config.LoadEnvFile(configPath); err == nil {
		return values, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	merged := map[string]string{}
	for _, path := range []string{legacyWrapperConfigPath, legacyServicesConfigPath} {
		values, err := config.LoadEnvFile(path)
		if err == nil {
			for key, value := range values {
				merged[key] = value
			}
			continue
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return merged, nil
}

func cleanupLegacyConfigPath(path, unifiedPath string) {
	if strings.TrimSpace(path) == "" || samePath(path, unifiedPath) {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return
	}
}
