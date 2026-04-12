#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="${ROOT_DIR}/bin"
BUILD_OUTPUT="${BIN_DIR}/codex-remote"
GO_BIN="${GO_BIN:-go}"
BASE_DIR=""
INSTANCE=""
UPGRADE_SLOT=""
ALLOW_DIRTY=0
BASE_DIR_SET=0

usage() {
  cat <<'EOF'
usage: ./upgrade-local.sh [--instance <id>] [--base-dir <dir>] [--slot <slot>] [--allow-dirty]

Pull the current branch to the latest upstream commit, rebuild ./bin/codex-remote,
stage it into the fixed local-upgrade artifact path, and trigger the built-in
local upgrade transaction against the installed daemon state.

options:
  --instance <id>   install instance to upgrade (default: workspace-bound instance or stable)
  --base-dir <dir>  base dir used by the local install state (default: workspace binding, detected global instance, or $HOME)
  --slot <slot>     optional explicit upgrade slot label
  --allow-dirty     skip the clean-worktree guard before git pull
  -h, --help        show this help text
EOF
}

normalize_instance() {
  local instance="$1"
  if [[ -z "${instance}" || "${instance}" == "stable" ]]; then
    printf 'stable'
    return
  fi
  printf '%s' "${instance}"
}

instance_namespace() {
  local instance="$1"
  instance="$(normalize_instance "${instance}")"
  if [[ "${instance}" == "stable" ]]; then
    printf 'codex-remote'
    return
  fi
  printf 'codex-remote-%s' "${instance}"
}

instance_state_root() {
  local base_dir="$1"
  local instance="$2"
  local namespace
  namespace="$(instance_namespace "${instance}")"
  if [[ "${instance}" == "stable" ]]; then
    printf '%s/.local/share/%s' "${base_dir}" "${namespace}"
    return
  fi
  printf '%s/.local/share/%s/codex-remote' "${base_dir}" "${namespace}"
}

instance_state_path() {
  local base_dir="$1"
  local instance="$2"
  printf '%s/install-state.json' "$(instance_state_root "${base_dir}" "${instance}")"
}

instance_config_path() {
  local base_dir="$1"
  local instance="$2"
  local namespace
  instance="$(normalize_instance "${instance}")"
  namespace="$(instance_namespace "${instance}")"
  if [[ "${instance}" == "stable" ]]; then
    printf '%s/.config/%s/config.json' "${base_dir}" "${namespace}"
    return
  fi
  printf '%s/.config/%s/codex-remote/config.json' "${base_dir}" "${namespace}"
}

binding_json_file() {
  printf '%s/.codex-remote/install-target.json' "${ROOT_DIR}"
}

binding_field() {
  local field="$1"
  local path
  path="$(binding_json_file)"
  [[ -f "${path}" ]] || return 1
  command -v python3 >/dev/null 2>&1 || return 1
  python3 - "${path}" "${field}" <<'PY'
import json
import sys

path, field = sys.argv[1], sys.argv[2]
with open(path, "r", encoding="utf-8") as fh:
    data = json.load(fh)
value = data.get(field, "")
if isinstance(value, str):
    sys.stdout.write(value)
PY
}

find_existing_base_dir() {
  local instance="$1"
  local current="${ROOT_DIR}"
  while true; do
    if [[ -f "$(instance_state_path "${current}" "${instance}")" || -f "$(instance_config_path "${current}" "${instance}")" ]]; then
      printf '%s' "${current}"
      return 0
    fi
    if [[ "${current}" == "/" ]]; then
      return 1
    fi
    current="$(dirname "${current}")"
  done
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --instance)
      [[ $# -ge 2 ]] || { echo "missing value for --instance" >&2; exit 1; }
      INSTANCE="$2"
      shift 2
      ;;
    --base-dir)
      [[ $# -ge 2 ]] || { echo "missing value for --base-dir" >&2; exit 1; }
      BASE_DIR="$2"
      BASE_DIR_SET=1
      shift 2
      ;;
    --slot)
      [[ $# -ge 2 ]] || { echo "missing value for --slot" >&2; exit 1; }
      UPGRADE_SLOT="$2"
      shift 2
      ;;
    --allow-dirty)
      ALLOW_DIRTY=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "${INSTANCE}" ]]; then
  if binding_instance="$(binding_field instanceId 2>/dev/null)"; then
    INSTANCE="${binding_instance}"
  else
    repo_instance_file="${ROOT_DIR}/.codex-remote/install-instance"
    if [[ -f "${repo_instance_file}" ]]; then
      INSTANCE="$(tr -d '[:space:]' < "${repo_instance_file}")"
    fi
  fi
fi
INSTANCE="${INSTANCE:-stable}"
INSTANCE="$(normalize_instance "${INSTANCE}")"

if [[ "${BASE_DIR_SET}" != "1" ]]; then
  if binding_base_dir="$(binding_field baseDir 2>/dev/null)" && [[ -n "${binding_base_dir}" ]]; then
    BASE_DIR="${binding_base_dir}"
  elif detected_base_dir="$(find_existing_base_dir "${INSTANCE}")"; then
    BASE_DIR="${detected_base_dir}"
  else
    BASE_DIR="${HOME}"
  fi
fi

cd "${ROOT_DIR}"

if [[ "${ALLOW_DIRTY}" != "1" ]]; then
  if ! git diff --quiet --ignore-submodules -- || ! git diff --cached --quiet --ignore-submodules --; then
    echo "working tree has uncommitted changes; commit/stash them first or rerun with --allow-dirty" >&2
    exit 1
  fi
fi

state_root="$(instance_state_root "${BASE_DIR}" "${INSTANCE}")"
state_path="$(instance_state_path "${BASE_DIR}" "${INSTANCE}")"
artifact_dir="${state_root}/local-upgrade"
artifact_path="${artifact_dir}/codex-remote"

printf '[1/4] git pull --ff-only\n'
git pull --ff-only

printf '[2/4] build %s\n' "${BUILD_OUTPUT}"
mkdir -p "${BIN_DIR}"
CLOUDFLARED_EMBED_ALLOW_DOWNLOAD=0 \
  bash "${ROOT_DIR}/scripts/externalaccess/prepare-cloudflared-embed.sh"
"${GO_BIN}" build -o "${BUILD_OUTPUT}" "${ROOT_DIR}/cmd/codex-remote"

if [[ ! -f "${state_path}" ]]; then
  echo "install state not found: ${state_path}" >&2
  echo "build ./bin/codex-remote and run './bin/codex-remote install -bootstrap-only -start-daemon' first, or pass --base-dir for the installed environment" >&2
  exit 1
fi

printf '[3/4] stage local artifact %s\n' "${artifact_path}"
mkdir -p "${artifact_dir}"
cp "${BUILD_OUTPUT}" "${artifact_path}"
chmod +x "${artifact_path}"

printf '[4/4] request built-in local upgrade transaction\n'
unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY all_proxy

cmd=("${BUILD_OUTPUT}" local-upgrade "-instance" "${INSTANCE}" "-base-dir" "${BASE_DIR}")
if [[ -n "${UPGRADE_SLOT}" ]]; then
  cmd+=("-slot" "${UPGRADE_SLOT}")
fi
CODEX_REMOTE_REPO_ROOT="${ROOT_DIR}" "${cmd[@]}"
