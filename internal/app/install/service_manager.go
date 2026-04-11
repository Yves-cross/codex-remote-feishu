package install

import (
	"fmt"
	"path/filepath"
	"strings"
)

type ServiceManager string

const (
	ServiceManagerDetached    ServiceManager = "detached"
	ServiceManagerSystemdUser ServiceManager = "systemd_user"
)

type installLayout struct {
	ConfigDir string
	StateDir  string
	StatePath string
}

func ParseServiceManager(value, goos string) (ServiceManager, error) {
	switch normalizeServiceManager(ServiceManager(value)) {
	case ServiceManagerDetached:
		return ServiceManagerDetached, nil
	case ServiceManagerSystemdUser:
		if goos != "linux" {
			return "", fmt.Errorf("service manager %q is only supported on linux", ServiceManagerSystemdUser)
		}
		return ServiceManagerSystemdUser, nil
	default:
		return "", fmt.Errorf("unsupported service manager %q (want detached or systemd_user)", strings.TrimSpace(value))
	}
}

func normalizeServiceManager(value ServiceManager) ServiceManager {
	switch strings.ToLower(strings.TrimSpace(string(value))) {
	case "", string(ServiceManagerDetached):
		return ServiceManagerDetached
	case string(ServiceManagerSystemdUser):
		return ServiceManagerSystemdUser
	default:
		return ""
	}
}

func effectiveServiceManager(state InstallState) ServiceManager {
	if normalized := normalizeServiceManager(state.ServiceManager); normalized != "" {
		return normalized
	}
	return ServiceManagerDetached
}

func installLayoutForBaseDir(baseDir string) installLayout {
	baseDir = filepath.Clean(strings.TrimSpace(baseDir))
	configDir := filepath.Join(baseDir, ".config", "codex-remote")
	stateDir := filepath.Join(baseDir, ".local", "share", "codex-remote")
	return installLayout{
		ConfigDir: configDir,
		StateDir:  stateDir,
		StatePath: filepath.Join(stateDir, "install-state.json"),
	}
}

func defaultInstallStatePath(baseDir string) string {
	return installLayoutForBaseDir(baseDir).StatePath
}

func defaultConfigPath(baseDir string) string {
	return filepath.Join(installLayoutForBaseDir(baseDir).ConfigDir, "config.json")
}

func baseDirFromConfigPath(path string) (string, bool) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return "", false
	}
	dir := filepath.Dir(path)
	if filepath.Base(dir) != "codex-remote" {
		return "", false
	}
	configHome := filepath.Dir(dir)
	if filepath.Base(configHome) != ".config" {
		return "", false
	}
	return filepath.Dir(configHome), true
}

func baseDirFromInstallStatePath(path string) (string, bool) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return "", false
	}
	dir := filepath.Dir(path)
	if filepath.Base(dir) != "codex-remote" {
		return "", false
	}
	dataHome := filepath.Dir(dir)
	if filepath.Base(dataHome) != "share" {
		return "", false
	}
	localHome := filepath.Dir(dataHome)
	if filepath.Base(localHome) != ".local" {
		return "", false
	}
	return filepath.Dir(localHome), true
}

func inferBaseDir(configPath, statePath string) string {
	if baseDir, ok := baseDirFromInstallStatePath(statePath); ok {
		return baseDir
	}
	if baseDir, ok := baseDirFromConfigPath(configPath); ok {
		return baseDir
	}
	return ""
}

func systemdUserUnitPath(baseDir string) string {
	baseDir = filepath.Clean(strings.TrimSpace(baseDir))
	if baseDir == "" {
		return ""
	}
	return filepath.Join(baseDir, ".config", "systemd", "user", "codex-remote.service")
}
