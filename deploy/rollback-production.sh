#!/usr/bin/env bash
# Atomically roll back V2UI current release to prior release (or explicit release ID).
set -euo pipefail

prefix="/opt/v2ui"
releases_dir="$prefix/releases"
current_pointer="$prefix/current"
prior_pointer="$prefix/prior"
target_release=""

usage() {
  cat <<'USAGE'
Usage:
  deploy/rollback-production.sh [options]

Options:
  --release-id ID   Target release ID (default: target pointed by /opt/v2ui/prior)
  --root DIR        Root directory prefix (default: /)
  -h, --help        Show this help.
USAGE
}

root="/"
while (($#)); do
  case "$1" in
    --release-id)
      (($# >= 2)) || { echo "--release-id requires a value" >&2; exit 1; }
      target_release=$2
      shift 2
      ;;
    --root)
      (($# >= 2)) || { echo "--root requires a value" >&2; exit 1; }
      root=$2
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

if [[ "$root" != "/" ]]; then
  prefix="${root%/}/opt/v2ui"
  releases_dir="$prefix/releases"
  current_pointer="$prefix/current"
  prior_pointer="$prefix/prior"
fi

[[ -d "$prefix" && -d "$releases_dir" ]] || {
  echo "rollback-production: releases directory $releases_dir does not exist" >&2
  exit 1
}

[[ -L "$current_pointer" ]] || {
  echo "rollback-production: current pointer $current_pointer is missing or not a symlink" >&2
  exit 1
}

current_target="$(readlink "$current_pointer")"
current_id="${current_target#releases/}"

if [[ -z "$target_release" ]]; then
  [[ -L "$prior_pointer" ]] || {
    echo "rollback-production: prior pointer $prior_pointer is missing or not a symlink" >&2
    exit 1
  }
  prior_target="$(readlink "$prior_pointer")"
  target_release="${prior_target#releases/}"
fi

target_path="$releases_dir/$target_release"
[[ -d "$target_path" && -f "$target_path/bin/v2ui-backend" ]] || {
  echo "rollback-production: release $target_release does not exist or lacks bin/v2ui-backend" >&2
  exit 1
}

if [[ "$target_release" == "$current_id" ]]; then
  echo "rollback-production: target release is already current ($current_id)"
  exit 0
fi

# Atomic pointer update
temp_curr="$prefix/.current.new.$$"
temp_prior="$prefix/.prior.new.$$"
ln -s "releases/$target_release" "$temp_curr"
ln -s "releases/$current_id" "$temp_prior"
mv -Tf "$temp_curr" "$current_pointer"
mv -Tf "$temp_prior" "$prior_pointer"

echo "Rolled back V2UI: current is now $target_release (prior: $current_id)"
