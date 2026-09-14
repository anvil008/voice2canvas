#!/usr/bin/env bash
# Install voice2canvas systemd user units. Run as the login user (no sudo).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

mkdir -p ~/.config/systemd/user
sed "s|%h/repos/voice2canvas|${REPO_ROOT}|g" "$SCRIPT_DIR/voice2canvas-backend.service" > ~/.config/systemd/user/voice2canvas-backend.service
sed "s|%h/repos/voice2canvas|${REPO_ROOT}|g" "$SCRIPT_DIR/voice2canvas-frontend.service" > ~/.config/systemd/user/voice2canvas-frontend.service

systemctl --user daemon-reload
systemctl --user enable --now voice2canvas-backend.service voice2canvas-frontend.service
# Keep user services running when not logged in:
loginctl enable-linger "$USER" 2>/dev/null || true
systemctl --user --no-pager status voice2canvas-backend.service voice2canvas-frontend.service | head -20
