#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FORMAT="text"

usage() {
  cat <<'EOF'
usage: bash scripts/dev/worktree-facts.sh [--format text|shell]

Emit stable worktree and publish facts for this repository.

Fields:
  root
  branch
  head
  upstream
  tracked_dirty
  untracked
  clean
  ahead
  behind
  publish_action
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --format)
      [[ $# -ge 2 ]] || { echo "missing value for --format" >&2; exit 1; }
      FORMAT="$2"
      shift 2
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

cd "${ROOT_DIR}"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "not inside a git worktree: ${ROOT_DIR}" >&2
  exit 1
fi

branch="$(git branch --show-current 2>/dev/null || true)"
head="$(git rev-parse HEAD)"
upstream="$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null || true)"

tracked_dirty=0
if ! git diff --quiet --ignore-submodules -- || ! git diff --cached --quiet --ignore-submodules --; then
  tracked_dirty=1
fi

untracked="$(git ls-files --others --exclude-standard | wc -l | tr -d ' ')"
clean="false"
if [[ "${tracked_dirty}" == "0" && "${untracked}" == "0" ]]; then
  clean="true"
fi

ahead=""
behind=""
publish_action=""
if [[ -z "${upstream}" ]]; then
  publish_action="set-upstream"
elif [[ "${clean}" != "true" ]]; then
  publish_action="blocked-dirty"
else
  counts="$(git rev-list --left-right --count "HEAD...${upstream}")"
  read -r ahead behind <<< "${counts}"
  case "${ahead}:${behind}" in
    0:0)
      publish_action="noop"
      ;;
    0:*)
      publish_action="sync-before-push"
      ;;
    *:0)
      publish_action="push"
      ;;
    *)
      publish_action="rebase-then-push"
      ;;
  esac
fi

emit_text() {
  cat <<EOF
root: ${ROOT_DIR}
branch: ${branch}
head: ${head}
upstream: ${upstream}
tracked_dirty: ${tracked_dirty}
untracked: ${untracked}
clean: ${clean}
ahead: ${ahead}
behind: ${behind}
publish_action: ${publish_action}
EOF
}

emit_shell() {
  cat <<EOF
root=$(printf '%q' "${ROOT_DIR}")
branch=$(printf '%q' "${branch}")
head=$(printf '%q' "${head}")
upstream=$(printf '%q' "${upstream}")
tracked_dirty=$(printf '%q' "${tracked_dirty}")
untracked=$(printf '%q' "${untracked}")
clean=$(printf '%q' "${clean}")
ahead=$(printf '%q' "${ahead}")
behind=$(printf '%q' "${behind}")
publish_action=$(printf '%q' "${publish_action}")
EOF
}

case "${FORMAT}" in
  text)
    emit_text
    ;;
  shell)
    emit_shell
    ;;
  *)
    echo "unsupported format: ${FORMAT}" >&2
    exit 1
    ;;
esac
