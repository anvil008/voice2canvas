#!/usr/bin/env bash
set -Eeuo pipefail

archive="$(realpath -m "$PWD/$1")"
checksum="$(realpath -m "$PWD/$2")"
status_file="$(realpath -m "$PWD/$3")"
temp_root="$(mktemp -d "$PWD/.native-workspace.XXXXXXXX")"
workspace="$temp_root/v2ui"
mkdir -p "$workspace"
trap 'rm -rf -- "$temp_root"' EXIT
rsync -aL --exclude='/.native-workspace.*' --exclude='/bazel-*' "$PWD/" "$workspace/"
cd "$workspace"

export CI=true
export PATH="/usr/local/go/bin:$PATH"
export HOME="${HOME:-/var/lib/gha-v2ui}"
export GOPATH="${HOME}/go"
export GOMODCACHE="${GOPATH}/pkg/mod"
export GOCACHE="${HOME}/.cache/go-build"
mkdir -p "$GOMODCACHE" "$GOCACHE"
(cd backend && go vet ./... && go test ./... && CGO_ENABLED=0 go build -trimpath -o "$workspace/v2ui-backend" ./cmd/server)
(cd frontend && npm ci && npm test && npm run build)

mkdir -p "$(dirname "$archive")"
stage="$(mktemp -d "$workspace/release.XXXXXXXX")"
install -Dm0755 v2ui-backend "$stage/bin/v2ui-backend"
cp -a frontend/dist "$stage/static"
cp -a deploy "$stage/deploy"
commit="$(awk '$1 == "STABLE_GIT_COMMIT" {print $2}' "$status_file")"
printf 'commit=%s\n' "$commit" >"$stage/release-manifest.txt"
tar -C "$stage" -czf "$archive" .
printf '%s  %s\n' "$(sha256sum "$archive" | awk '{print $1}')" "$(basename "$archive")" >"$checksum"
