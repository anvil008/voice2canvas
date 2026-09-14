# Voice2Canvas

Voice2Canvas is an open reference implementation of V2UI, the voice-to-UI framework described in the paper [When voice builds the interface](https://anvilpalamattam.com/papers/when-voice-builds-the-interface/): turning a live voice conversation into a dynamic interface. A user speaks to Gemini Live, an ADK agent converts the request into bounded tasks, and those tasks generate validated A2UI cards on a responsive canvas.

Voice2Canvas provides a clean, modular voice-to-interface pipeline with explicit local setup, configuration, and bring-your-own-key testing.

## What it demonstrates

```text
microphone → Gemini Live → ADK dispatch tool → card agents → A2UI validation → React canvas
```

- Full-duplex browser audio over WebSocket
- Fast task acknowledgement with concurrent card generation
- Server-side A2UI schema validation and client-side defensive validation
- A small extended catalog for stats, charts, gauges, progress, key/value data, badges, and tables
- Per-session Gemini API keys entered in the browser, or normal server-side environment/ADC authentication
- A deterministic `?mock=1` mode that needs no key

## API Prerequisites

Voice2Canvas uses Gemini Live (`gemini-3.1-flash-live-preview`) for bidirectional voice communication and `gemini-3.6-flash` for A2UI card generation.

- **Get an API Key**: Obtain a Gemini API key from [Google AI Studio](https://aistudio.google.com/app/apikey).
- **Model Access**: Ensure your account has access to Gemini Live models.
- **Google Cloud / Vertex AI (Alternative)**: Vertex AI Application Default Credentials (ADC) are also supported with `GOOGLE_CLOUD_PROJECT` and `GOOGLE_CLOUD_LOCATION`.

## Quick start

Requirements: Go 1.26+, Node.js 22+, and a modern browser.

### Terminal 1: Frontend

```sh
cd frontend
npm ci
npm run dev
```

### Terminal 2: Backend

```sh
cd backend
go run ./cmd/server
```

Open <https://localhost:5173> (or `https://<lan-ip>:5173` from another device on your local network). The Vite dev server runs over HTTPS by default to satisfy browser secure context requirements for microphone capture on LAN devices without needing Chrome flags. On the first visit to the LAN IP, browsers display a self-signed certificate warning: click "Advanced" -> "Proceed to <ip> (unsafe)", after which the browser treats the origin as secure and prompts for normal microphone permissions. HTTPS can be disabled if needed by setting `HTTPS=false`, `VOICE2CANVAS_HTTPS=0`, or `V2UI_HTTPS=0`.

Click the connect icon in the top-right corner to add a Gemini API key, or press the microphone and Voice2Canvas will ask for one when the server has no credentials. The key stays in the current tab's memory and is sent in the WebSocket `start` frame; it is not saved to browser storage.

To explore the UI without Gemini, open <https://localhost:5173/?mock=1>.

### Server-managed authentication

For a shared development server, set `GEMINI_API_KEY` before starting the Go backend:

```sh
GEMINI_API_KEY=... go run ./cmd/server
```

When both are present, a browser-provided key applies only to that WebSocket session and takes precedence over server credentials.

## Repository layout

- `frontend/` — React, Vite, audio capture/playback, A2UI renderer, and canvas
- `backend/` — Go HTTP/WebSocket server, Gemini Live session, generalized ADK specialists for weather, markets, and research, plus schema validation
- `catalog/` — the portable extended A2UI catalog contract
- `PROTOCOL.md` — browser/server wire protocol
- `docs/` — operations and deployment documentation

## Verify

```sh
cd frontend && npm ci && npm test && npm run typecheck && npm run build
cd ../backend && go test ./... && go vet ./... && go build ./cmd/server
```

## Security

The browser-key flow is designed for a local demo you run yourself. A deployed backend necessarily receives the key and could observe it, so do not paste a key into an instance operated by someone you do not trust. See [`SECURITY.md`](SECURITY.md) for the threat model and deployment guidance.

## Status

Voice2Canvas is an experimental reference project, not a hosted service or stable SDK. The protocol and extended catalog are expected to evolve.

## License

MIT — see [`LICENSE`](LICENSE).
