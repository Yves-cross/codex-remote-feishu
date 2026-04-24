#!/usr/bin/env bash
set -euo pipefail

CHECK=""
ARGS=()

usage() {
  cat <<'EOF'
usage: bash scripts/dev/gh-json-fields.sh [--check field1,field2] <gh-subcommand> [<arg>...]

Examples:
  bash scripts/dev/gh-json-fields.sh issue view 1
  bash scripts/dev/gh-json-fields.sh --check number,title,state issue view 1

The script first tries `gh ... --help`, then falls back to a runtime probe with
an invalid JSON field. Pass enough arguments for the target subcommand to run.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check)
      [[ $# -ge 2 ]] || { echo "missing value for --check" >&2; exit 1; }
      CHECK="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      ARGS+=("$1")
      shift
      ;;
  esac
done

if [[ ${#ARGS[@]} -eq 0 ]]; then
  usage >&2
  exit 1
fi

extract_fields_from_help() {
  local help_output="$1"
  printf '%s\n' "${help_output}" | awk '
    /^JSON Fields$/ {capture=1; next}
    capture && NF==0 {exit}
    capture && /^[A-Z][A-Za-z ]+$/ {exit}
    capture {print}
  '
}

extract_fields_from_probe() {
  local probe_output="$1"
  printf '%s\n' "${probe_output}" | awk '
    /^Available fields:$/ {capture=1; next}
    capture && /^[[:space:]]+[A-Za-z0-9]/ {print; next}
    capture {exit}
  '
}

help_output="$(gh "${ARGS[@]}" --help)"
fields_raw="$(extract_fields_from_help "${help_output}")"
if [[ -z "${fields_raw}" ]]; then
  probe_output="$(gh "${ARGS[@]}" --json __invalid__ 2>&1 || true)"
  fields_raw="$(extract_fields_from_probe "${probe_output}")"
fi

if [[ -z "${fields_raw}" ]]; then
  echo "could not determine JSON fields for: gh ${ARGS[*]}" >&2
  echo "pass enough arguments for the target subcommand or inspect `gh ${ARGS[*]} --help` manually" >&2
  exit 2
fi

mapfile -t fields < <(
  printf '%s\n' "${fields_raw}" |
    tr ',' '\n' |
    sed 's/^[[:space:]]*//; s/[[:space:]]*$//' |
    awk 'NF > 0' |
    sort -u
)

if [[ ${#fields[@]} -eq 0 ]]; then
  echo "no supported JSON fields parsed for: gh ${ARGS[*]}" >&2
  exit 2
fi

echo "command: gh ${ARGS[*]}"
echo "supported_json_fields:"
printf '  %s\n' "${fields[@]}"

if [[ -n "${CHECK}" ]]; then
  declare -A supported=()
  for field in "${fields[@]}"; do
    supported["${field}"]=1
  done
  IFS=',' read -r -a requested <<< "${CHECK}"
  missing=()
  for field in "${requested[@]}"; do
    field="$(echo "${field}" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')"
    [[ -n "${field}" ]] || continue
    if [[ -z "${supported[${field}]:-}" ]]; then
      missing+=("${field}")
    fi
  done
  if [[ ${#missing[@]} -gt 0 ]]; then
    echo "unsupported_json_fields:" >&2
    printf '  %s\n' "${missing[@]}" >&2
    exit 3
  fi
fi
