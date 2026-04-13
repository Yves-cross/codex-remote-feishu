package upgradeshim

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/app/install"
	"github.com/kxn/codex-remote-feishu/internal/upgradeshim"
)

func RunMain(args []string) int {
	if len(args) != 0 {
		_, _ = fmt.Fprintf(os.Stderr, "upgrade shim: unexpected arguments: %s\n", strings.Join(args, " "))
		return 1
	}
	executable, err := os.Executable()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "upgrade shim: resolve executable: %v\n", err)
		return 1
	}
	statePath, err := resolveStatePath(executable)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "upgrade shim: %v\n", err)
		return 1
	}
	if err := install.RunUpgradeHelperWithStatePath(context.Background(), statePath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "upgrade shim: %v\n", err)
		return 1
	}
	return 0
}

func resolveStatePath(entrypointPath string) (string, error) {
	entrypointPath = filepath.Clean(strings.TrimSpace(entrypointPath))
	if entrypointPath == "" {
		return "", fmt.Errorf("entrypoint path is empty")
	}
	sidecarPath := upgradeshim.SidecarPath(entrypointPath)
	sidecar, err := upgradeshim.ReadSidecar(sidecarPath)
	if err != nil {
		return "", err
	}
	if !upgradeshim.SidecarValid(sidecar) {
		return "", fmt.Errorf("upgrade shim sidecar is invalid")
	}
	return filepath.Clean(strings.TrimSpace(sidecar.InstallStatePath)), nil
}
