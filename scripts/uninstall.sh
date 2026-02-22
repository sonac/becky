#!/usr/bin/env bash
set -euo pipefail

BIN_PATH="/usr/local/bin/mongo-backup"

if systemctl list-unit-files | grep -q '^mongo-backup.service'; then
  sudo systemctl disable --now mongo-backup || true
  sudo rm -f /etc/systemd/system/mongo-backup.service
  sudo systemctl daemon-reload
fi

sudo rm -f "$BIN_PATH"

echo "mongo-backup removed. Data under ~/.becky was left intact."
