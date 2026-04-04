#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

bump="${BUMP_OVERRIDE:-auto}"
case "${bump}" in
  auto|patch|minor|major) ;;
  *)
    echo "Unsupported bump override: ${bump}" >&2
    exit 1
    ;;
esac

latest_tag="$(git tag --list 'v*' --sort=-version:refname | head -n1 || true)"

if [[ -z "${latest_tag}" ]]; then
  case "${bump}" in
    auto|minor)
      echo "v0.1.0"
      ;;
    patch)
      echo "v0.0.1"
      ;;
    major)
      echo "v1.0.0"
      ;;
  esac
  exit 0
fi

version_core="${latest_tag#v}"
IFS=. read -r major minor patch <<<"${version_core}"

if [[ -z "${major:-}" || -z "${minor:-}" || -z "${patch:-}" ]]; then
  echo "Latest tag is not a semantic version: ${latest_tag}" >&2
  exit 1
fi

commit_range="${latest_tag}..HEAD"
if [[ -z "$(git log --format='%H' "${commit_range}")" ]]; then
  echo "No unreleased commits since ${latest_tag}." >&2
  exit 1
fi

if [[ "${bump}" == "auto" ]]; then
  subjects="$(git log --format='%s' "${commit_range}")"
  bodies="$(git log --format='%B' "${commit_range}")"

  if grep -Eq 'BREAKING CHANGE|^[^[:space:]]+(\(.+\))?!:' <<<"${bodies}"$'\n'"${subjects}"; then
    bump="major"
  elif grep -Eq '^(feat(\(.+\))?:|Add |Implement |Create |Support )' <<<"${subjects}"; then
    bump="minor"
  else
    bump="patch"
  fi
fi

case "${bump}" in
  major)
    major=$((major + 1))
    minor=0
    patch=0
    ;;
  minor)
    minor=$((minor + 1))
    patch=0
    ;;
  patch)
    patch=$((patch + 1))
    ;;
esac

printf 'v%s.%s.%s\n' "${major}" "${minor}" "${patch}"
