package upgradeshim

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/upgradeshim"
)

func TestResolveStatePathUsesSidecar(t *testing.T) {
	dir := t.TempDir()
	entrypoint := filepath.Join(dir, "upgrade-helper", "codex-remote-upgrade-shim")
	statePath := filepath.Join(dir, "install-state.json")
	if err := os.MkdirAll(filepath.Dir(entrypoint), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(statePath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := upgradeshim.WriteSidecar(upgradeshim.SidecarPath(entrypoint), upgradeshim.Sidecar{
		InstallStatePath: statePath,
		InstanceID:       "stable",
	}); err != nil {
		t.Fatalf("WriteSidecar: %v", err)
	}

	got, err := resolveStatePath(entrypoint)
	if err != nil {
		t.Fatalf("resolveStatePath: %v", err)
	}
	if got != statePath {
		t.Fatalf("state path = %q, want %q", got, statePath)
	}
}
