# Voice2Canvas as a container

Production V2UI is the image `ghcr.io/anvil008/v2ui`, built by
[`.github/workflows/docker.yml`](../../.github/workflows/docker.yml) on every push to
`main` and deployed onto the apps VM (`10.0.20.200`) by forge's `compose-deploy`
workflow from `compose/apps/v2ui/compose.yaml`.

This replaces the release-tarball path documented in
[`production-runbook.md`](../production-runbook.md), which installed
`/opt/v2ui/releases/<id>` on CT 110 over SSH. CT 110 is stopped as of 2026-09-08 and
`.github/workflows/deploy.yml` is a `workflow_dispatch`-only stub that fails loudly.
`deploy/deploy-release.sh`, `deploy/install-production.sh` and
`deploy/rollback-production.sh` are kept as the record of that path; nothing runs them.
`deploy/verify-production.sh` still works against `--host 10.0.20.200 --port 8083`.

## The backend now serves the SPA

Before this image, nothing did. `build/bazel/ci.sh` copied `frontend/dist` into the release
tarball as `static/`, but `v2ui.service` never set a static directory and the Go mux only
registered `/healthz`, `/ws`, `/api/agents` and `/debug/cards` — so `GET /` returned 404 in
production and the UI only ever ran from `vite dev` against a separately started backend.

`backend/internal/server/static.go` is a port of the equivalent file from the ghost fork of
this codebase, function for function. It is wired to `VOICE2CANVAS_STATIC_DIR` (or `V2UI_STATIC_DIR`):

- unset — the `/` route is not registered at all and behaviour is exactly as before, which
  is what local backend-only development wants;
- set to something unusable — `NewHandler` returns an error and the process exits at
  startup rather than serving a 404 wall;
- set to a directory with a readable `index.html` — that directory is served, with SPA
  fallback to `index.html` for unknown paths, *except* that a path that looks like a
  fingerprinted asset (`…-a1b2c3d4.js`) 404s instead of returning HTML, and `/api`, `/ws`,
  `/healthz` and `/debug` are never shadowed by a file on disk.

The image bakes `VOICE2CANVAS_STATIC_DIR=/opt/v2ui/static` (`V2UI_STATIC_DIR=/opt/v2ui/static`) and ships the built bundle there.

## Tags

| Tag | When |
|---|---|
| `latest` | every push to `main` |
| short SHA, e.g. `909bdea` | every push to `main` and every tag |
| `vYYYY.MM.DD[.N]` | a published git tag matching `v*` |

`latest` is pinned to `main` by `flavor: latest=false` plus a main-only `type=raw` entry.
The package is **private**; the VM 200 docker daemon needs a `read:packages` login as the
`deploy` user.

## Port

**8083**, unchanged from CT 110, inside the container and on `10.0.20.200`.

The image sets `PORT` *and* `V2UI_LISTEN_ADDR` because `DefaultListenAddr` in this repo is
`:8080`, which on VM 200 is Open WebUI. A container that fell back to the default would
either fail to publish or collide, so neither variable is left to chance.
`resolveListenAddr` consults `VOICE2CANVAS_LISTEN_ADDR` first, then `V2UI_LISTEN_ADDR`, then `PORT`.

## State

**There is none.** V2UI writes no files — no database, no state directory, no embedded
store. CT 110's `/var/lib/v2ui` was empty at cutover and the compose project declares no
volume. Recreating the container loses nothing.

## Environment contract

Baked into the image: `HOME=/var/lib/v2ui`, `PORT=8083`,
`V2UI_LISTEN_ADDR=0.0.0.0:8083`, `VOICE2CANVAS_STATIC_DIR=/opt/v2ui/static` (`V2UI_STATIC_DIR=/opt/v2ui/static`).

The backend reads exactly seven variables. That is the whole surface — this fork has
neither seaglass's `internal/infra` nor its `internal/codingterminal`, so the
`V2UI_LOKI_URL`, `V2UI_PROM_URL`, `V2UI_PVE_*`, `V2UI_CONTROL_TOKEN` and `V2UI_CODE_ROOTS`
entries in `deploy/v2ui.env.example` were copied from seaglass and are **inert here**.
They are repointed anyway, because a stale address in an example file is a trap.

| Variable | On CT 110 | Meaning |
|---|---|---|
| `GEMINI_API_KEY` | **empty** | Gemini Live and card generation. Empty is carried forward faithfully: v2ui never had a key on CT 110, so `authConfigured` is false and the voice path is inert until one is supplied. The SPA and `/api/agents` work without it. |
| `VOICE2CANVAS_LIVE_MODEL` / `V2UI_LIVE_MODEL` | `gemini-3.1-flash-live-preview` | falls back to `DefaultLiveModel` when empty |
| `VOICE2CANVAS_CARD_MODEL` / `V2UI_CARD_MODEL` | `gemini-3.6-flash` | falls back to `DefaultCardModel` when empty |
| `VOICE2CANVAS_LISTEN_ADDR`, `V2UI_LISTEN_ADDR`, `PORT` | `0.0.0.0:8083`, `8083` | baked in |
| `VOICE2CANVAS_STATIC_DIR`, `V2UI_STATIC_DIR` | *(did not exist)* | baked in |
| `GOOGLE_CLOUD_PROJECT`, `GOOGLE_CLOUD_LOCATION`, `GOOGLE_CLOUD_REGION` | empty | optional Vertex AI path, unused |

## The WebSocket origin check is open, and there is no variable for it

`server.go`'s upgrader is:

```go
CheckOrigin: func(_ *http.Request) bool {
    // This is a local test dashboard; deployments should put an
    // origin policy in front of it.
    return true
},
```

Every origin is accepted and **no environment variable changes that** — unlike the ghost
and seaglass forks, this one has no `V2UI_ALLOWED_ORIGINS` support at all. The posture is
identical to what CT 110 ran, so moving to VM 200 is not a regression, but it is worth
saying plainly: anything on the LAN that can reach `10.0.20.200:8083` can drive the
WebSocket, and the code comment above is an unfulfilled instruction, not a description of
what is deployed. Putting an origin policy in front of it — or porting seaglass's
allowlist — is the obvious follow-up.

## Health

`GET /healthz` is served by the Go mux and does not touch the network. The image's
`HEALTHCHECK` curls it every 30 s.

## Building locally

```sh
docker build -t v2ui:dev .
docker run --rm -p 8083:8083 v2ui:dev
curl -fsS http://127.0.0.1:8083/healthz
curl -fsS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8083/
```

The build is the same two commands `build/bazel/ci.sh` runs natively — `npm ci && npm run
build` in `frontend/`, `go build ./cmd/server` in `backend/` — so a Bazel CI failure and a
Docker build failure have the same cause.
