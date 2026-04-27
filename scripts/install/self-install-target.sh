#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
GO_BIN="${GO_BIN:-go}"

cd "${ROOT_DIR}"
"${GO_BIN}" run ./scripts/install/self-install-target "$@"
