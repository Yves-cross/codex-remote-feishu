#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

usage() {
  cat <<'EOF'
usage: bash scripts/dev/code-line-stats.sh

Count tracked project lines by file type in two scopes:
1. all tracked recognized text files
2. excluding tests and docs

Rules:
- file set comes from `git ls-files`
- line count uses physical lines (`awk 'END { print NR }'`)
- docs excluded from the second scope:
  - `docs/**`
  - `*.md`
- tests excluded from the second scope:
  - `testkit/**`
  - `**/testdata/**`
  - `**/__tests__/**`
  - `*_test.go`
  - `*.test.*`
  - `*.spec.*`
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

declare -A ALL_FILE_COUNTS=()
declare -A ALL_LINE_COUNTS=()
declare -A CORE_FILE_COUNTS=()
declare -A CORE_LINE_COUNTS=()

all_total_files=0
all_total_lines=0
core_total_files=0
core_total_lines=0
recognized_files=0
skipped_files=0

classify_type() {
  local path="$1"
  local base="${path##*/}"

  case "$path" in
    .githooks/*)
      echo "Shell"
      return 0
      ;;
  esac

  case "$base" in
    Dockerfile)
      echo "Dockerfile"
      return 0
      ;;
    Makefile)
      echo "Makefile"
      return 0
      ;;
  esac

  case "$path" in
    *.go) echo "Go" ;;
    *.ts) echo "TypeScript" ;;
    *.tsx) echo "TSX" ;;
    *.js|*.mjs|*.cjs) echo "JavaScript" ;;
    *.jsx) echo "JSX" ;;
    *.sh|*.bash) echo "Shell" ;;
    *.ps1) echo "PowerShell" ;;
    *.json) echo "JSON" ;;
    *.yml|*.yaml) echo "YAML" ;;
    *.html) echo "HTML" ;;
    *.css|*.scss|*.sass|*.less) echo "CSS" ;;
    *.md) echo "Markdown" ;;
    *.mod) echo "Go Module" ;;
    *.sum) echo "Go Sum" ;;
    *.example) echo "Example" ;;
    .gitignore|.dockerignore) echo "Dotfile" ;;
    *)
      return 1
      ;;
  esac
}

is_doc_file() {
  local path="$1"
  case "$path" in
    docs/*|*.md)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

is_test_file() {
  local path="$1"
  case "$path" in
    testkit/*|*/testdata/*|*/__tests__/*|*_test.go|*.test.*|*.spec.*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

count_lines() {
  local path="$1"
  awk 'END { print NR + 0 }' "$path"
}

append_count() {
  local scope="$1"
  local type="$2"
  local lines="$3"

  case "$scope" in
    all)
      ALL_FILE_COUNTS["$type"]=$(( ${ALL_FILE_COUNTS["$type"]:-0} + 1 ))
      ALL_LINE_COUNTS["$type"]=$(( ${ALL_LINE_COUNTS["$type"]:-0} + lines ))
      all_total_files=$((all_total_files + 1))
      all_total_lines=$((all_total_lines + lines))
      ;;
    core)
      CORE_FILE_COUNTS["$type"]=$(( ${CORE_FILE_COUNTS["$type"]:-0} + 1 ))
      CORE_LINE_COUNTS["$type"]=$(( ${CORE_LINE_COUNTS["$type"]:-0} + lines ))
      core_total_files=$((core_total_files + 1))
      core_total_lines=$((core_total_lines + lines))
      ;;
    *)
      echo "unknown scope: ${scope}" >&2
      exit 1
      ;;
  esac
}

write_rows() {
  local scope="$1"
  local dest="$2"
  : >"${dest}"

  case "$scope" in
    all)
      for type in "${!ALL_LINE_COUNTS[@]}"; do
        printf '%s\t%d\t%d\n' "${type}" "${ALL_FILE_COUNTS["$type"]}" "${ALL_LINE_COUNTS["$type"]}" >>"${dest}"
      done
      ;;
    core)
      for type in "${!CORE_LINE_COUNTS[@]}"; do
        printf '%s\t%d\t%d\n' "${type}" "${CORE_FILE_COUNTS["$type"]}" "${CORE_LINE_COUNTS["$type"]}" >>"${dest}"
      done
      ;;
  esac
}

print_report() {
  local title="$1"
  local rows_file="$2"
  local total_files="$3"
  local total_lines="$4"

  printf '\n== %s ==\n' "${title}"
  printf '%-16s %8s %10s\n' "Type" "Files" "Lines"
  printf '%-16s %8s %10s\n' "----" "-----" "-----"
  while IFS=$'\t' read -r type files lines; do
    [[ -n "${type}" ]] || continue
    printf '%-16s %8d %10d\n' "${type}" "${files}" "${lines}"
  done < <(sort -t $'\t' -k3,3nr -k1,1 "${rows_file}")
  printf '%-16s %8s %10s\n' "----" "-----" "-----"
  printf '%-16s %8d %10d\n' "TOTAL" "${total_files}" "${total_lines}"
}

while IFS= read -r -d '' path; do
  if ! file_type="$(classify_type "${path}")"; then
    skipped_files=$((skipped_files + 1))
    continue
  fi
  recognized_files=$((recognized_files + 1))
  lines="$(count_lines "${path}")"
  append_count all "${file_type}" "${lines}"

  if is_doc_file "${path}" || is_test_file "${path}"; then
    continue
  fi
  append_count core "${file_type}" "${lines}"
done < <(git ls-files -z)

all_rows="$(mktemp)"
core_rows="$(mktemp)"
trap 'rm -f "${all_rows}" "${core_rows}"' EXIT

write_rows all "${all_rows}"
write_rows core "${core_rows}"

printf 'Repository: %s\n' "${ROOT_DIR}"
printf 'Tracked files: %d recognized, %d skipped (unclassified)\n' "${recognized_files}" "${skipped_files}"

print_report "All Recognized Files (includes tests/docs)" "${all_rows}" "${all_total_files}" "${all_total_lines}"
print_report "Exclude Tests/Docs" "${core_rows}" "${core_total_files}" "${core_total_lines}"
