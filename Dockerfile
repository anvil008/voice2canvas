# Voice2Canvas / V2UI as a container image, replacing the CT 110 release-tarball deployment.
#
# Three stages, matching what build/bazel/ci.sh does natively:
#   1. `npm ci && npm run build` in frontend/  -> frontend/dist
#   2. `go build ./cmd/server` in backend/     -> a static binary
#   3. a small runtime that carries both
#
# The SPA is not embedded. internal/server reads V2UI_STATIC_DIR at startup and
# refuses to serve `/` unless that directory holds a readable index.html, so the
# built assets ship as files at /opt/v2ui/static and the variable is baked in
# below. Before this image, nothing served them: the release tarball carried a
# `static/` directory that v2ui.service never pointed at, so the UI root 404'd
# in production and the SPA only ever ran from `vite dev`.
#
# Final stage is debian:bookworm-slim so this image, seaglass's and ghost's --
# the same codebase, three forks -- share a base. `curl` is present for the
# HEALTHCHECK.

# Stage 1: build the frontend bundle.
FROM node:22-bookworm-slim AS web-builder

WORKDIR /app/frontend

# Dependency manifests first so `npm ci` is cached independently of the sources.
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

# `npm run build` is `npm run typecheck && vite build`; it needs tsconfig.json,
# vite.config.ts, index.html and public/ as well as src/.
COPY frontend/ ./
RUN npm run build


# Stage 2: build the Go binary.
FROM golang:1.26-bookworm AS go-builder

# backend/go.mod pins `go 1.26.5`; let the toolchain fetch an exact match rather
# than failing when the base image tag floats to a different patch release.
ENV GOTOOLCHAIN=auto

WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

# CGO_ENABLED=0 and -trimpath match build/bazel/ci.sh.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /v2ui-backend ./cmd/server


# Stage 3: runtime.
FROM debian:bookworm-slim AS runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

# uid/gid 10001, the same fixed non-system uid nexus, alpha-core, ghost and
# seaglass use.
RUN groupadd --system --gid 10001 v2ui \
    && useradd --system --uid 10001 --gid v2ui \
       --home-dir /var/lib/v2ui --shell /usr/sbin/nologin v2ui \
    && mkdir -p /var/lib/v2ui \
    && chown -R v2ui:v2ui /var/lib/v2ui

COPY --from=go-builder /v2ui-backend /usr/local/bin/v2ui-backend
COPY --from=web-builder --chown=root:root /app/frontend/dist /opt/v2ui/static
RUN chmod -R a+rX /opt/v2ui/static

# 8083 is the port CT 110's v2ui.service bound and the port this keeps on
# VM 200. The image's own DefaultListenAddr is :8080, which would collide with
# Open WebUI, so both PORT and V2UI_LISTEN_ADDR are set explicitly --
# resolveListenAddr consults V2UI_LISTEN_ADDR first, then PORT.
ENV HOME=/var/lib/v2ui \
    PORT=8083 \
    VOICE2CANVAS_LISTEN_ADDR=0.0.0.0:8083 \
    V2UI_LISTEN_ADDR=0.0.0.0:8083 \
    VOICE2CANVAS_STATIC_DIR=/opt/v2ui/static \
    V2UI_STATIC_DIR=/opt/v2ui/static

WORKDIR /var/lib/v2ui
USER v2ui:v2ui

EXPOSE 8083

# /healthz is served by the Go mux and does not touch the network, so a green
# check means the process is up, not that Gemini is reachable.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fsS "http://127.0.0.1:${VOICE2CANVAS_LISTEN_ADDR##*:}/healthz" || curl -fsS "http://127.0.0.1:${V2UI_LISTEN_ADDR##*:}/healthz" || exit 1

ENTRYPOINT ["/usr/local/bin/v2ui-backend"]
