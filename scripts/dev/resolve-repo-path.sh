#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

usage() {
  cat <<'EOF'
usage: bash scripts/dev/resolve-repo-path.sh <repo-path> [<repo-path>...]

Resolve one or more repository-relative paths to absolute paths.
If a path does not exist, print a few likely repo matches and exit non-zero.
EOF
}

if [[ $# -eq 0 ]]; then
  usage >&2
  exit 1
fi

cd "${ROOT_DIR}"

resolve_one() {
  local raw="$1"
  local candidate=""
  if [[ "${raw}" = /* ]]; then
    candidate="${raw}"
  else
    candidate="${ROOT_DIR}/${raw#./}"
  fi
  if [[ -e "${candidate}" ]]; then
    readlink -f "${candidate}"
    return 0
  fi

  local base stem
  base="$(basename "${raw}")"
  stem="${base%.*}"
  echo "path not found under repo root: ${raw}" >&2

  local suggestions=()
  if [[ -n "${base}" ]]; then
    mapfile -t suggestions < <(rg --files | rg --fixed-strings -- "${base}" | head -n 10)
  fi
  if [[ ${#suggestions[@]} -eq 0 && -n "${stem}" && "${stem}" != "${base}" ]]; then
    mapfile -t suggestions < <(rg --files | rg --fixed-strings -- "${stem}" | head -n 10)
  fi
  if [[ ${#suggestions[@]} -gt 0 ]]; then
    echo "did you mean:" >&2
    printf '  %s\n' "${suggestions[@]}" >&2
  fi
  return 2
}

status=0
for raw in "$@"; do
  if ! resolve_one "${raw}"; then
    status=$?
  fi
done
exit "${status}"
