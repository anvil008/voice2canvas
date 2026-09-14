# Running Voice2Canvas with Docker

Voice2Canvas can be built and run as a single container packaging both the Go backend and the compiled React frontend single-page application.

## Quickstart

### 1. Build the Docker image

```bash
docker build -t voice2canvas:latest .
```

### 2. Run the container

Run the container with your Gemini API key:

```bash
docker run -d \
  --name voice2canvas \
  -p 8080:8080 \
  -e GEMINI_API_KEY="your-gemini-api-key" \
  voice2canvas:latest
```

The application will be accessible at `http://localhost:8080`.

### 3. Verify Health

Check that the server is healthy:

```bash
curl -fsS http://localhost:8080/healthz
```

Expected response: `ok`.

## Docker Compose

You can also run Voice2Canvas using Docker Compose. Create a `compose.yaml` file:

```yaml
services:
  voice2canvas:
    build: .
    image: voice2canvas:latest
    ports:
      - "8080:8080"
    environment:
      - GEMINI_API_KEY=${GEMINI_API_KEY}
      - VOICE2CANVAS_LIVE_MODEL=gemini-3.1-flash-live-preview
      - VOICE2CANVAS_CARD_MODEL=gemini-3.6-flash
      - LOG_LEVEL=info
      - LOG_FORMAT=json
    restart: unless-stopped
```

Start the service:

```bash
docker compose up -d
```

## Environment Configuration

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Port for the HTTP and WebSocket server. |
| `VOICE2CANVAS_LISTEN_ADDR` | `0.0.0.0:8080` | Host and port to listen on. |
| `VOICE2CANVAS_STATIC_DIR` | `/opt/voice2canvas/static` | Path to compiled frontend assets inside the container. |
| `GEMINI_API_KEY` | *(empty)* | API key for Gemini Live and Gemini Card generation models. |
| `VOICE2CANVAS_LIVE_MODEL` | `gemini-3.1-flash-live-preview` | Model used for bidirectional voice sessions. |
| `VOICE2CANVAS_CARD_MODEL` | `gemini-3.6-flash` | Model used for A2UI card generation. |
| `VOICE2CANVAS_ALLOWED_ORIGINS` | *(empty)* | Comma-separated list of allowed origins for WebSocket connections (e.g. `https://example.com`). If empty, requests matching the `Host` header or empty Origin are allowed. |
| `GOOGLE_CLOUD_PROJECT` | *(empty)* | Optional Google Cloud project ID when using Vertex AI credentials. |
| `GOOGLE_CLOUD_LOCATION` | *(empty)* | Optional Google Cloud region when using Vertex AI credentials. |
| `LOG_LEVEL` | `info` | Logging verbosity (`debug`, `info`, `warn`, `error`). |
| `LOG_FORMAT` | `text` | Logging format (`text` or `json`). |

## Architecture & Container Structure

- **Frontend Builder Stage**: Builds the Vite single-page application using Node.js.
- **Backend Builder Stage**: Compiles the Go server into `/usr/local/bin/voice2canvas-server`.
- **Runtime Stage**: A minimal Linux environment running as non-root user `voice2canvas:voice2canvas`, serving static assets directly and managing the WebSocket endpoint at `/ws`.
- **Health Check**: An integrated `HEALTHCHECK` periodically verifies `http://127.0.0.1:8080/healthz`.
