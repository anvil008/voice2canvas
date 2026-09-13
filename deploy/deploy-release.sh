#!/usr/bin/env bash
# Ship a CI-built release archive to the V2UI host. The archive is staged as
# an immutable /opt/v2ui/releases/<commit> tree, current/prior are swapped
# atomically, v2ui.service is restarted, and the previous release is restored
# if the new one does not come back healthy.
#
# Invoked by .github/workflows/deploy.yml through the Forge reusable deploy
# workflow, which exports COMMIT for the commit being deployed. The host is the
# DEPLOY_HOST repository variable: V2UI has no provisioned production host
# yet, and this script installs no user, unit or environment file — it replaces
# the application artifact on an already installed host and nothing more.
set -Eeuo pipefail

readonly archive="${1:?usage: deploy-release.sh ARCHIVE HOST}"
readonly host="${2:?set the DEPLOY_HOST repository variable to the v2ui host}"
readonly commit="${COMMIT:?COMMIT is exported by the Forge deploy workflow}"

[[ -f "$archive" && ! -L "$archive" ]] || {
  echo "release artifact is not a regular file" >&2
  exit 1
}
[[ "$host" =~ ^[A-Za-z0-9][A-Za-z0-9.-]*$ ]] || {
  echo "deploy host is not a bare hostname or address" >&2
  exit 1
}
[[ "$commit" =~ ^[0-9a-f]{40}$ ]] || {
  echo "COMMIT is not a full git object name" >&2
  exit 1
}

scp -o BatchMode=yes -o StrictHostKeyChecking=accept-new \
  "$archive" "root@${host}:/root/v2ui-release.tar.gz"
ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new "root@${host}" \
  "COMMIT=${commit} bash -se" <<'REMOTE'
set -Eeuo pipefail

readonly prefix=/opt/v2ui
readonly releases="$prefix/releases"
readonly current="$prefix/current"
readonly prior="$prefix/prior"
readonly release="$releases/$COMMIT"
readonly staged=/root/v2ui-release.tar.gz
readonly unit=v2ui.service
readonly health=http://127.0.0.1:8083/healthz
incoming="$releases/.$COMMIT.incoming.$$"
trap 'rm -rf -- "$staged" ${incoming:+"$incoming"} \
  "$prefix/.current.new.$$" "$prefix/.prior.new.$$"' EXIT

# An atomic relative pointer swap: current and prior always name releases/<id>.
point() {
  local pointer=$1 id=$2
  local temporary="$prefix/.${pointer##*/}.new.$$"
  ln -s "releases/$id" "$temporary"
  mv -Tf -- "$temporary" "$pointer"
}

healthy() {
  local _attempt
  for _attempt in {1..15}; do
    if systemctl is-active --quiet "$unit" &&
       curl --fail --silent --show-error --max-time 3 "$health" >/dev/null; then
      return 0
    fi
    sleep 1
  done
  return 1
}

[[ -f "$staged" && ! -L "$staged" ]] || {
  echo 'staged V2UI archive is not a regular file' >&2
  exit 1
}
[[ -d "$releases" && ! -L "$releases" ]] || {
  echo 'V2UI release hierarchy is missing; provision the host first' >&2
  exit 1
}

previous=""
if [[ -L "$current" ]]; then
  previous="$(readlink -- "$current")"
  previous="${previous#releases/}"
  [[ -d "$releases/$previous" && ! -L "$releases/$previous" ]] || {
    echo 'current pointer does not name a release directory' >&2
    exit 1
  }
elif [[ -e "$current" ]]; then
  echo 'current pointer exists but is not a symlink' >&2
  exit 1
fi

# Recorded so that a rolled-back deployment leaves both pointers exactly as it
# found them, rather than leaving prior naming the release it restored.
fallback=""
if [[ -L "$prior" ]]; then
  fallback="$(readlink -- "$prior")"
  fallback="${fallback#releases/}"
fi

rm -rf -- "$incoming"
mkdir -m 0755 -- "$incoming"
tar --extract --gzip --file "$staged" --directory "$incoming" \
  --no-same-owner --no-same-permissions
[[ -f "$incoming/bin/v2ui-backend" && -f "$incoming/static/index.html" ]] || {
  echo 'release archive is missing bin/v2ui-backend or static/index.html' >&2
  exit 1
}
chown -R root:root -- "$incoming"
find "$incoming" -type d -exec chmod 0755 {} +
find "$incoming" -type f -exec chmod 0644 {} +
chmod 0755 "$incoming/bin/v2ui-backend"

# Redeploying the running commit keeps its tree in place; the restart below
# still picks up a changed unit or environment file.
if [[ "$previous" != "$COMMIT" ]]; then
  rm -rf -- "$release"
  mv -T -- "$incoming" "$release"
  incoming=""
  [[ -z "$previous" ]] || point "$prior" "$previous"
  point "$current" "$COMMIT"
fi

if systemctl restart "$unit" && healthy; then
  exit 0
fi

echo "V2UI $COMMIT failed health verification; restoring the previous release" >&2
[[ -n "$previous" && "$previous" != "$COMMIT" ]] || {
  echo 'no previous V2UI release to restore' >&2
  exit 1
}
point "$current" "$previous"
[[ -z "$fallback" ]] || point "$prior" "$fallback"
systemctl restart "$unit"
if healthy; then
  echo "Previous V2UI release $previous restored; deployment remains failed" >&2
  exit 1
fi
echo 'Automatic V2UI rollback did not recover service health' >&2
exit 1
REMOTE
