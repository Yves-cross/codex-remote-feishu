package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/upgradeshim"
	upgradeshimembed "github.com/kxn/codex-remote-feishu/internal/upgradeshim/embed"
)

type UpgradeShimEntrypointOptions struct {
	EntrypointPath   string
	InstallStatePath string
	InstanceID       string
}

func UpgradeShimSidecarPath(entrypointPath string) string {
	return upgradeshim.SidecarPath(entrypointPath)
}

func WriteUpgradeShimEntrypoint(opts UpgradeShimEntrypointOptions) error {
	entrypointPath := strings.TrimSpace(opts.EntrypointPath)
	if entrypointPath == "" {
		return fmt.Errorf("upgrade shim entrypoint path is required")
	}
	sidecar := upgradeshim.Sidecar{
		InstallStatePath: opts.InstallStatePath,
		InstanceID:       opts.InstanceID,
	}
	if !upgradeshim.SidecarValid(sidecar) {
		return fmt.Errorf("upgrade shim install requires install state path")
	}
	if err := os.MkdirAll(filepath.Dir(entrypointPath), 0o755); err != nil {
		return err
	}
	if err := upgradeshimembed.WriteExecutable(entrypointPath); err != nil {
		return err
	}
	return upgradeshim.WriteSidecar(UpgradeShimSidecarPath(entrypointPath), sidecar)
}
