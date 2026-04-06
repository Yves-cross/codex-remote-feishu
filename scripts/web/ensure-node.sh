#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
NODE_VERSION="${NODE_VERSION:-v24.14.1}"
NODE_DIR="$ROOT_DIR/.tools/node/$NODE_VERSION"
NODE_LINK="$ROOT_DIR/.tools/node/current"

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
  Linux) os_name="linux" ;;
  Darwin) os_name="darwin" ;;
  *)
    echo "unsupported node platform: $os" >&2
    exit 1
    ;;
esac

case "$arch" in
  x86_64|amd64) arch_name="x64" ;;
  arm64|aarch64) arch_name="arm64" ;;
  *)
    echo "unsupported node architecture: $arch" >&2
    exit 1
    ;;
esac

platform="$os_name-$arch_name"
archive="node-$NODE_VERSION-$platform.tar.xz"
url="https://nodejs.org/dist/$NODE_VERSION/$archive"

if [[ ! -x "$NODE_DIR/bin/node" ]]; then
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT
  mkdir -p "$NODE_DIR"
  curl -fsSL "$url" -o "$tmp_dir/$archive"
  tar -xJf "$tmp_dir/$archive" -C "$tmp_dir"
  rm -rf "$NODE_DIR"
  mv "$tmp_dir/node-$NODE_VERSION-$platform" "$NODE_DIR"
fi

mkdir -p "$(dirname "$NODE_LINK")"
ln -sfn "$NODE_DIR" "$NODE_LINK"
echo "$NODE_DIR"
