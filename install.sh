#!/usr/bin/env bash
set -euo pipefail

REPO="BillioncodesInc/club"
APP_NAME="phishingclub"

echo "Getting Phishing Club from $REPO"

# Get first phishingclub*.tar.gz asset from latest release
URL=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep -Eo "https://[^\"]+/releases/download/[^\"]+/${APP_NAME}[^\"/]*\.tar\.gz" | head -1) || true
[ -n "$URL" ] || { echo "[!] No ${APP_NAME} tarball found in ${REPO} releases" >&2; exit 1; }

TMP=$(mktemp -d /tmp/${APP_NAME}.XXXXXX)
curl -fsSL "$URL" -o "$TMP/pc.tgz"

# Extract (flat archive: binary lands directly inside $TMP)
tar -xzf "$TMP/pc.tgz" -C "$TMP"

echo "Installing from $TMP"
cd "$TMP"
sudo ./${APP_NAME} --install
