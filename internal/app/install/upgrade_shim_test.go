package install

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	upgradeshimembed "github.com/kxn/codex-remote-feishu/internal/upgradeshim/embed"
)

func TestWriteUpgradeShimEntrypointWritesExecutableAndSidecar(t *testing.T) {
	if _, ok := upgradeshimembed.Current(); !ok {
		t.Fatal("expected embedded upgrade shim asset for host platform")
	}

	dir := t.TempDir()
	entrypoint := filepath.Join(dir, "upgrade-helper", executableName(runtime.GOOS))
	statePath := filepath.Join(dir, "install-state.json")
	if err := os.WriteFile(statePath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile state: %v", err)
	}

	if err := WriteUpgradeShimEntrypoint(UpgradeShimEntrypointOptions{
		EntrypointPath:   entrypoint,
		InstallStatePath: statePath,
		InstanceID:       "stable",
	}); err != nil {
		t.Fatalf("WriteUpgradeShimEntrypoint: %v", err)
	}

	raw, err := os.ReadFile(entrypoint)
	if err != nil {
		t.Fatalf("ReadFile executable: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("expected extracted shim executable to be non-empty")
	}
	sidecarRaw, err := os.ReadFile(UpgradeShimSidecarPath(entrypoint))
	if err != nil {
		t.Fatalf("ReadFile sidecar: %v", err)
	}
	if !bytes.Contains(sidecarRaw, []byte(statePath)) {
		t.Fatalf("sidecar = %q, want state path", string(sidecarRaw))
	}
}
