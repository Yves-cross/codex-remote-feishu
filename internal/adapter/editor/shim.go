package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func PatchBundleEntrypoint(entrypointPath, wrapperBinary string) error {
	if strings.TrimSpace(entrypointPath) == "" {
		return fmt.Errorf("bundle entrypoint path is required")
	}
	if strings.TrimSpace(wrapperBinary) == "" {
		return fmt.Errorf("wrapper binary path is required")
	}
	if err := os.MkdirAll(filepath.Dir(entrypointPath), 0o755); err != nil {
		return err
	}
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REAL_BIN="${SCRIPT_DIR}/codex.real"
WRAPPER_BIN=%q

if [[ ! -x "$REAL_BIN" ]]; then
    echo "Missing original Codex binary: $REAL_BIN" >&2
    exit 1
fi

if [[ ! -x "$WRAPPER_BIN" ]]; then
    echo "Missing relay wrapper binary: $WRAPPER_BIN" >&2
    exit 1
fi

export CODEX_REAL_BINARY="${CODEX_REAL_BINARY:-$REAL_BIN}"
export CODEX_RELAY_WRAPPER_INTEGRATION_MODE="${CODEX_RELAY_WRAPPER_INTEGRATION_MODE:-managed_shim}"

exec "$WRAPPER_BIN" "$@"
`, wrapperBinary)
	return os.WriteFile(entrypointPath, []byte(script), 0o755)
}
