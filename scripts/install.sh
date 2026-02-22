#!/usr/bin/env bash
set -euo pipefail

REPO="sonac/becky"
BIN_NAME="mongo-backup"
ASSET="mongo-backup_linux_amd64.tar.gz"
SUMS="sha256sums.txt"
BIN_DIR="/usr/local/bin"

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

VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | awk -F '"' '/tag_name/{print $4; exit}')"

if [[ -z "$VERSION" ]]; then
  echo "Unable to resolve latest release version" >&2
  exit 1
fi

BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

echo "Downloading ${VERSION}..."
curl -fsSL "${BASE_URL}/${ASSET}" -o "${WORKDIR}/${ASSET}"
curl -fsSL "${BASE_URL}/${SUMS}" -o "${WORKDIR}/${SUMS}"

pushd "$WORKDIR" >/dev/null
grep " ${ASSET}$" "${SUMS}" | sha256sum -c -
tar -xzf "${ASSET}"
popd >/dev/null

sudo install -m 0755 "${WORKDIR}/${BIN_NAME}" "${BIN_DIR}/${BIN_NAME}"

echo "Installed ${BIN_NAME} ${VERSION}"
echo "Binary: ${BIN_DIR}/${BIN_NAME}"
echo "Next: run '${BIN_DIR}/${BIN_NAME} init --help'"
