#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
TARGET_SCRIPT="${ROOT_DIR}/scripts/install/self-install-target.sh"

usage() {
  cat <<'EOF'
usage: scripts/install/self-target-request.sh <admin|tool> <path> [curl args...]

Resolve the current daemon self target from the active runtime environment,
then issue a localhost HTTP request to that instance.

examples:
  scripts/install/self-target-request.sh admin /v1/status
  scripts/install/self-target-request.sh admin /api/admin/bootstrap-state
  scripts/install/self-target-request.sh tool /healthz
  scripts/install/self-target-request.sh admin /v1/status | jq .
EOF
}

if [[ $# -lt 2 ]]; then
  usage >&2
  exit 1
fi

target_kind="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
path="${2:-}"
shift 2

case "${target_kind}" in
  admin|tool) ;;
  -h|--help)
    usage
    exit 0
    ;;
  *)
    echo "unsupported target kind: ${target_kind}" >&2
    usage >&2
    exit 1
    ;;
esac

if [[ -z "${path}" ]]; then
  echo "missing request path" >&2
  usage >&2
  exit 1
fi
if [[ "${path}" != /* ]]; then
  path="/${path}"
fi

eval "$("${TARGET_SCRIPT}" --format shell)"

case "${target_kind}" in
  admin)
    base_url="${CODEX_REMOTE_SELF_TARGET_ADMIN_URL:-}"
    ;;
  tool)
    base_url="${CODEX_REMOTE_SELF_TARGET_TOOL_URL:-}"
    ;;
esac

if [[ -z "${base_url}" ]]; then
  echo "resolved current self target has no ${target_kind} URL; resolve with scripts/install/self-install-target.sh first" >&2
  exit 1
fi

unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY all_proxy
exec curl --noproxy '*' -fsS "$@" "${base_url%/}${path}"
