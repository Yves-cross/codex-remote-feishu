#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
NODE_DIR="$(bash "$ROOT_DIR/scripts/web/ensure-node.sh")"
export PATH="$NODE_DIR/bin:$PATH"

cd "$ROOT_DIR/web"
if [[ -f package-lock.json ]]; then
  npm ci || npm install
else
  npm install
fi
npm run build
