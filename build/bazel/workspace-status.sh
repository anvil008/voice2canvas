#!/usr/bin/env bash
set -Eeuo pipefail
printf 'STABLE_GIT_COMMIT %s\n' "$(git rev-parse HEAD 2>/dev/null || printf unknown)"
printf 'STABLE_SOURCE_DATE_EPOCH %s\n' "$(git show -s --format=%ct HEAD 2>/dev/null || printf 0)"
