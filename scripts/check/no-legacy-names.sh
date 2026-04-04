#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

patterns=(
  'module '"fschannel"
  '"fs'channel/
  'CODEX_'RELAY_
  'codex-'relay
  'go-'rewrite-architecture\.md
  '/data/dl/'fschannel
  '/home/dl/'fschannel
)
pattern="$(IFS='|'; printf '%s' "${patterns[*]}")"

if rg -n "${pattern}" README.md DEVELOPER.md .env.example Makefile install.sh cmd internal docs .github; then
  echo "Found legacy project names or deprecated paths." >&2
  exit 1
fi
