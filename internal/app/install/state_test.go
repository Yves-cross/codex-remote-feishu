package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadStateCollapsesLegacyConfigPaths(t *testing.T) {
	baseDir := t.TempDir()
	statePath := filepath.Join(baseDir, ".local", "share", "codex-remote", "install-state.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	legacyWrapperPath := filepath.Join(baseDir, ".config", "codex-remote", "wrapper.env")
	legacyServicesPath := filepath.Join(baseDir, ".config", "codex-remote", "services.env")
	raw := `{
  "statePath": "` + statePath + `",
  "wrapperConfigPath": "` + legacyWrapperPath + `",
  "servicesConfigPath": "` + legacyServicesPath + `"
}`
	if err := os.WriteFile(statePath, []byte(raw), 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}

	loaded, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	wantConfigPath := filepath.Join(baseDir, ".config", "codex-remote", "config.json")
	if loaded.ConfigPath != wantConfigPath {
		t.Fatalf("ConfigPath = %q, want %q", loaded.ConfigPath, wantConfigPath)
	}
	if loaded.StatePath != statePath {
		t.Fatalf("StatePath = %q, want %q", loaded.StatePath, statePath)
	}
}

func TestWriteStateOmitsLegacyConfigPathFields(t *testing.T) {
	baseDir := t.TempDir()
	statePath := filepath.Join(baseDir, ".local", "share", "codex-remote", "install-state.json")
	state := InstallState{
		BaseDir:    baseDir,
		ConfigPath: filepath.Join(baseDir, ".config", "codex-remote", "config.json"),
		StatePath:  statePath,
	}
	if err := WriteState(statePath, state); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	for _, field := range []string{"wrapperConfigPath", "servicesConfigPath"} {
		if strings.Contains(string(raw), field) {
			t.Fatalf("did not expect %s in written state: %s", field, raw)
		}
	}
}
