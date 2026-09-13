#!/usr/bin/env bash
# Verify Voice2Canvas / V2UI production service health and optional systemd status.
set -euo pipefail

host="127.0.0.1"
port="8083"
check_systemd=0
timeout=5

usage() {
  cat <<'USAGE'
Usage:
  deploy/verify-production.sh [options]

Options:
  --host HOST     Target hostname or IP (default: 127.0.0.1)
  --port PORT     Target port (default: 8083)
  --systemd       Verify voice2canvas.service / v2ui.service is active via systemctl
  --timeout SEC   Curl timeout in seconds (default: 5)
  -h, --help      Show this help.
USAGE
}

while (($#)); do
  case "$1" in
    --host)
      (($# >= 2)) || { echo "--host requires a value" >&2; exit 1; }
      host=$2
      shift 2
      ;;
    --port)
      (($# >= 2)) || { echo "--port requires a value" >&2; exit 1; }
      port=$2
      shift 2
      ;;
    --systemd)
      check_systemd=1
      shift
      ;;
    --timeout)
      (($# >= 2)) || { echo "--timeout requires a value" >&2; exit 1; }
      timeout=$2
      shift 2
      ;;
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

if (( check_systemd )); then
  if ! command -v systemctl >/dev/null 2>&1; then
    echo "verify-production: systemctl not available" >&2
    exit 1
  fi
  if ! systemctl is-active --quiet voice2canvas.service && ! systemctl is-active --quiet v2ui.service; then
    echo "verify-production: neither voice2canvas.service nor v2ui.service is active" >&2
    exit 1
  fi
  echo "voice2canvas.service / v2ui.service is active"
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "verify-production: curl command not available" >&2
  exit 1
fi

health_url="http://${host}:${port}/healthz"
if ! curl --fail --silent --show-error --max-time "$timeout" "$health_url" >/dev/null; then
  echo "verify-production: health check failed for $health_url" >&2
  exit 1
fi

echo "healthz check passed: $health_url"
