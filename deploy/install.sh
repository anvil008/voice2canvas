#!/usr/bin/env bash
# Install voice2canvas systemd user units. Run as the login user (no sudo).
set -euo pipefail
cd "$(dirname "$0")"

mkdir -p ~/.config/systemd/user
cp voice2canvas-backend.service voice2canvas-frontend.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now voice2canvas-backend.service voice2canvas-frontend.service
# Keep user services running when not logged in:
loginctl enable-linger "$USER" 2>/dev/null || true
systemctl --user --no-pager status voice2canvas-backend.service voice2canvas-frontend.service | head -20
