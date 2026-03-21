#!/usr/bin/env bash
set -euo pipefail

REPO="rnikoopour/mediocresync"
BINARY_NAME="mediocresync-linux-amd64"
INSTALL_PATH="/usr/local/bin/mediocresync"
UNIT_NAME="mediocresync"
UNIT_PATH="/etc/systemd/system/${UNIT_NAME}.service"

# Resolve latest release download URLs via GitHub API.
LATEST_URL="https://api.github.com/repos/${REPO}/releases/latest"
RELEASE_JSON=$(curl -fsSL "$LATEST_URL")
BINARY_URL=$(echo "$RELEASE_JSON" | grep -o "\"browser_download_url\": *\"[^\"]*${BINARY_NAME}\"" | grep -o 'https://[^"]*')
UNIT_URL=$(echo "$RELEASE_JSON"   | grep -o "\"browser_download_url\": *\"[^\"]*\.service\""      | grep -o 'https://[^"]*')

if [ -z "$BINARY_URL" ]; then
  echo "ERROR: could not find ${BINARY_NAME} in the latest release." >&2
  exit 1
fi
if [ -z "$UNIT_URL" ]; then
  echo "ERROR: could not find .service file in the latest release." >&2
  exit 1
fi

# Stop the unit if it is currently running.
if systemctl is-active --quiet "${UNIT_NAME}"; then
  echo "Stopping ${UNIT_NAME}..."
  systemctl stop "${UNIT_NAME}"
fi

# Download and install the binary.
echo "Downloading ${BINARY_URL}..."
curl -fsSL "$BINARY_URL" -o "$INSTALL_PATH"
chmod +x "$INSTALL_PATH"

# Download and install the systemd unit.
echo "Downloading ${UNIT_URL}..."
curl -fsSL "$UNIT_URL" -o "$UNIT_PATH"

# Reload systemd and (re)start the service.
systemctl daemon-reload
systemctl enable --now "${UNIT_NAME}"

echo "Done. ${UNIT_NAME} is running."
