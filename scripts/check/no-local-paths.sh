#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

if rg -n '/data/|/home/' README.md DEVELOPER.md docs .github --glob '*.md' --glob '*.yml'; then
  echo "Found machine-local absolute paths in public docs or workflow files." >&2
  exit 1
fi
