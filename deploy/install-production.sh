#!/usr/bin/env bash
# Install V2UI production systemd units, service account, and directories on CT110.
# Idempotent. Does not start the service if no binary exists yet.
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"

usage() {
  cat <<'USAGE'
Usage:
  deploy/install-production.sh [options]

Options:
  -h, --help    Show this help.
USAGE
}

while (($#)); do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

if (( EUID != 0 )); then
  echo "install-production: root privileges required" >&2
  exit 1
fi

for cmd in getent groupadd useradd install chown chmod mkdir systemctl; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "install-production: required command missing: $cmd" >&2
    exit 1
  fi
done

# Ensure system user and group
if ! getent group v2ui >/dev/null 2>&1; then
  groupadd --system v2ui
fi
if ! getent passwd v2ui >/dev/null 2>&1; then
  nologin="$(command -v nologin || echo /usr/sbin/nologin)"
  useradd --system --gid v2ui --home-dir /var/lib/v2ui --shell "$nologin" --no-create-home v2ui
fi

# Ensure directories
mkdir -p /opt/v2ui/releases
mkdir -p /opt/v2ui/runtime
mkdir -p /var/lib/v2ui
mkdir -p /etc/v2ui

chown root:root /opt/v2ui /opt/v2ui/releases
chmod 0755 /opt/v2ui /opt/v2ui/releases
chown v2ui:v2ui /opt/v2ui/runtime /var/lib/v2ui
chmod 0755 /opt/v2ui/runtime
chmod 0750 /var/lib/v2ui
chown root:v2ui /etc/v2ui
chmod 0750 /etc/v2ui

# Environment file
if [[ ! -f /etc/v2ui/v2ui.env ]]; then
  install -m 0640 -o root -g v2ui "$script_dir/v2ui.env.example" /etc/v2ui/v2ui.env
else
  chown root:v2ui /etc/v2ui/v2ui.env
  chmod 0640 /etc/v2ui/v2ui.env
fi

# Install systemd unit files
install -m 0644 -o root -g root "$script_dir/v2ui.service" /etc/systemd/system/v2ui.service
install -m 0644 -o root -g root "$script_dir/v2ui-edge-proxy.socket" /etc/systemd/system/v2ui-edge-proxy.socket
install -m 0644 -o root -g root "$script_dir/v2ui-edge-proxy.service" /etc/systemd/system/v2ui-edge-proxy.service

systemctl daemon-reload
systemctl enable v2ui.service

echo "V2UI production kit installed successfully."
