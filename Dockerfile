# Voice2Canvas multi-stage container build.
#
# Stages:
#   1. web-builder: Compiles the React SPA with Vite -> frontend/dist
#   2. go-builder: Compiles the Go server -> voice2canvas-server
#   3. runtime: Minimal Debian runtime serving the binary and static assets

# Stage 1: Build the frontend bundle.
FROM node:22-bookworm-slim AS web-builder

WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build


# Stage 2: Build the Go binary.
FROM golang:1.26-bookworm AS go-builder

ENV GOTOOLCHAIN=auto

WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /usr/local/bin/voice2canvas-server ./cmd/server


# Stage 3: Runtime.
FROM debian:bookworm-slim AS runtime

# Keep apt as root: rootless builds cannot map gid 65534 (nogroup), which apt otherwise drops to.
RUN echo 'APT::Sandbox::User "root";' > /etc/apt/apt.conf.d/99-rootless-build \
    && apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd --system --gid 10001 voice2canvas \
    && useradd --system --uid 10001 --gid voice2canvas \
       --home-dir /var/lib/voice2canvas --shell /usr/sbin/nologin voice2canvas \
    && mkdir -p /var/lib/voice2canvas \
    && chown -R voice2canvas:voice2canvas /var/lib/voice2canvas

COPY --from=go-builder /usr/local/bin/voice2canvas-server /usr/local/bin/voice2canvas-server
COPY --from=web-builder --chown=root:root /app/frontend/dist /opt/voice2canvas/static
RUN chmod -R a+rX /opt/voice2canvas/static

ENV HOME=/var/lib/voice2canvas \
    PORT=8080 \
    VOICE2CANVAS_LISTEN_ADDR=0.0.0.0:8080 \
    VOICE2CANVAS_STATIC_DIR=/opt/voice2canvas/static

WORKDIR /var/lib/voice2canvas
USER voice2canvas:voice2canvas

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fsS "http://127.0.0.1:${PORT}/healthz" || exit 1

ENTRYPOINT ["/usr/local/bin/voice2canvas-server"]
