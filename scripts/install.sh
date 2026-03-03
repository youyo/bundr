#!/usr/bin/env bash
# Usage:
#   curl -sSfL https://raw.githubusercontent.com/youyo/bundr/main/scripts/install.sh | bash
#   curl -sSfL https://raw.githubusercontent.com/youyo/bundr/main/scripts/install.sh | bash -s -- v0.5.0
#   INSTALL_DIR=/usr/local/bin sudo bash scripts/install.sh

set -euo pipefail

REPO="youyo/bundr"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin}"

# バージョン解決
VERSION="${1:-}"
if [ -z "${VERSION}" ]; then
  VERSION=$(curl -sSfL -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
fi
VERSION_NUM="${VERSION#v}"

# OS/ARCH 検出
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "${ARCH}" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported arch: ${ARCH}" >&2; exit 1 ;;
esac
case "${OS}" in
  linux|darwin) ;;
  *) echo "Unsupported OS: ${OS}" >&2; exit 1 ;;
esac

BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
ARCHIVE="bundr_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
CHECKSUM_FILE="bundr_${VERSION_NUM}_checksums.txt"

TMPDIR=$(mktemp -d)
trap "rm -rf ${TMPDIR}" EXIT

echo "Downloading bundr ${VERSION} (${OS}/${ARCH})..."
curl -sSfL "${BASE_URL}/${ARCHIVE}"       -o "${TMPDIR}/${ARCHIVE}"
curl -sSfL "${BASE_URL}/${CHECKSUM_FILE}" -o "${TMPDIR}/${CHECKSUM_FILE}"

# SHA256 検証
cd "${TMPDIR}"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum --ignore-missing -c "${CHECKSUM_FILE}"
elif command -v shasum >/dev/null 2>&1; then
  shasum -a 256 --ignore-missing -c "${CHECKSUM_FILE}"
else
  echo "WARNING: sha256sum/shasum not found, skipping verification" >&2
fi

# インストール
tar xzf "${ARCHIVE}"
mkdir -p "${INSTALL_DIR}"
install -m 755 bundr "${INSTALL_DIR}/bundr"

echo "bundr ${VERSION} installed to ${INSTALL_DIR}/bundr"
if ! echo "${PATH}" | grep -q "${INSTALL_DIR}"; then
  echo "Add to PATH: export PATH=\"${INSTALL_DIR}:\${PATH}\""
fi
