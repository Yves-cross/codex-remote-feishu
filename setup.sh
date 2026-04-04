#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="${ROOT_DIR}/bin"
GO_BIN="${GO_BIN:-go}"

mkdir -p "${BIN_DIR}"

"${GO_BIN}" build -o "${BIN_DIR}/codex-remote-relayd" "${ROOT_DIR}/cmd/relayd"
"${GO_BIN}" build -o "${BIN_DIR}/codex-remote-wrapper" "${ROOT_DIR}/cmd/relay-wrapper"
"${GO_BIN}" build -o "${BIN_DIR}/codex-remote-install" "${ROOT_DIR}/cmd/relay-install"

args=("$@")
if [[ ${#args[@]} -eq 0 ]]; then
  args=(-interactive)
fi

exec "${BIN_DIR}/codex-remote-install" "${args[@]}"
