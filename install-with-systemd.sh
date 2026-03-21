#!/usr/bin/env bash
set -euo pipefail

REPO="rnikoopour/mediocresync"
APP_NAME="mediocresync"
BINARY_NAME="${APP_NAME}-linux-amd64"
INSTALL_PATH="/usr/local/bin/${APP_NAME}"
UNIT_PATH="/etc/systemd/system/${APP_NAME}.service"

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

# Create the service user if it doesn't already exist.
if ! id -u "${APP_NAME}" &>/dev/null; then
  echo "Creating user ${APP_NAME}..."
  useradd -r -s /bin/false "${APP_NAME}"
fi

# Stop the unit if it is currently running.
if systemctl is-active --quiet "${APP_NAME}"; then
  echo "Stopping ${APP_NAME}..."
  systemctl stop "${APP_NAME}"
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
systemctl enable --now "${APP_NAME}"

echo "Done. ${APP_NAME} is running."
