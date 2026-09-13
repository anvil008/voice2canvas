# Voice2Canvas backend

The Go backend owns the Gemini Live session, ADK agent roster, asynchronous
card generation, A2UI validation, card registry, and WebSocket protocol.

## Run

```sh
go run ./cmd/server
```

The server listens on `http://localhost:8080`. It starts without credentials so
health checks and the frontend remain available; model-backed work begins only
after the browser supplies a per-session Gemini API key or server authentication
is configured.

```sh
curl http://localhost:8080/healthz
```

## Authentication

The browser may include an API key in its WebSocket `start` frame. A non-blank
per-session key takes precedence over server configuration, is used only to
construct that connection's model clients, and is not persisted or intentionally
logged. Only send a key to a backend you control.

Server-managed alternatives:

| Variable | Default | Purpose |
| --- | --- | --- |
| `GEMINI_API_KEY` | unset | Gemini API authentication |
| `GOOGLE_CLOUD_PROJECT` | unset | Vertex AI project for ADC |
| `GOOGLE_CLOUD_LOCATION` | unset | Vertex AI location for ADC |
| `VOICE2CANVAS_LIVE_MODEL` (or `V2UI_LIVE_MODEL`) | `gemini-3.1-flash-live-preview` | Live voice model |
| `VOICE2CANVAS_CARD_MODEL` (or `V2UI_CARD_MODEL`) | `gemini-3.6-flash` | Worker model |

## General agent roster

- **Voice front door** routes requests without blocking the conversation.
- **Weather specialist** resolves locations and fetches Open-Meteo conditions
  and forecasts.
- **Market specialist** uses grounded public search for stocks, indexes, funds,
  crypto, currencies, commodities, and company results.
- **Researcher** gathers sourced news and general facts.
- **Card generator** turns specialist briefs into validated A2UI cards.
- **Investigator** answers explain/why questions and sends a concise finding
  back through the voice session.
- **Layout curator** arranges the live canvas.
- **Auto-refresh** reruns eligible card pipelines on bounded intervals.

The front door assigns `domain: weather`, `domain: markets`, or
`domain: general`. Weather uses the allowlisted Open-Meteo function tool;
markets and general research use Google Search grounding. All series are capped
before they enter card state.

## Checks

```sh
go test ./...
go vet ./...
go build ./...
```
