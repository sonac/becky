#!/usr/bin/env bash
set -euo pipefail

REPO="sonac/becky"
BIN_NAME="mongo-backup"
ASSET="mongo-backup_linux_amd64.tar.gz"
SUMS="sha256sums.txt"
BIN_DIR="/usr/local/bin"
CURL_UA="Mozilla/5.0"

usage() {
  cat <<EOF
Usage: install.sh [--bin-dir /usr/local/bin]
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bin-dir)
      BIN_DIR="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

BASE_URL="https://github.com/${REPO}/releases/latest/download"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

download() {
  local url="$1"
  local out="$2"

  if curl -fsSL --retry 3 --retry-delay 1 --retry-all-errors "$url" -o "$out"; then
    return 0
  fi

  if curl -4fsSL --retry 3 --retry-delay 1 --retry-all-errors -A "$CURL_UA" "$url" -o "$out"; then
    return 0
  fi

  return 1
}

echo "Downloading latest release..."
download "${BASE_URL}/${ASSET}" "${WORKDIR}/${ASSET}" || {
  echo "Failed to download ${ASSET}" >&2
  exit 1
}
download "${BASE_URL}/${SUMS}" "${WORKDIR}/${SUMS}" || {
  echo "Failed to download ${SUMS}" >&2
  exit 1
}

pushd "$WORKDIR" >/dev/null
grep " ${ASSET}$" "${SUMS}" | sha256sum -c -
tar -xzf "${ASSET}"
popd >/dev/null

sudo install -m 0755 "${WORKDIR}/${BIN_NAME}" "${BIN_DIR}/${BIN_NAME}"

echo "Installed ${BIN_NAME} (latest release)"
echo "Binary: ${BIN_DIR}/${BIN_NAME}"
echo "Next: run '${BIN_DIR}/${BIN_NAME} init --help'"
