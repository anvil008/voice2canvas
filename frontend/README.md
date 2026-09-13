# Voice2Canvas frontend

Vite + React + TypeScript browser client for the repository's WebSocket protocol.

## Run

Start the Go backend on port 8080, then run:

```bash
cd frontend
npm ci
npm run dev
```

Open the URL Vite prints (normally `https://localhost:5173` or `https://<lan-ip>:5173`).
The Vite dev server runs over HTTPS by default to satisfy browser secure context
requirements for microphone capture on LAN devices without needing Chrome flags.
On the first visit to the LAN IP, browsers display a self-signed certificate warning:
click "Advanced" -> "Proceed to <ip> (unsafe)", after which the browser treats the
origin as secure and prompts for normal microphone permissions. HTTPS can be disabled
if needed by setting `HTTPS=false` or `VOICE2CANVAS_HTTPS=0` (or `V2UI_HTTPS=0`).

The dev server listens on the local network by default (`host: true`) and proxies `/ws`
(including WebSocket upgrades), `/healthz`, and `/api` to the Go backend. It respects
`VOICE2CANVAS_BACKEND_PORT` / `V2UI_BACKEND_PORT` or `PORT` (or `VOICE2CANVAS_BACKEND_URL` / `V2UI_BACKEND_URL`), defaulting to
`http://localhost:8080`.
Use the connect icon in the top-right corner to provide a per-session Gemini
API key. The key remains in tab memory and is never written to browser storage.

To demo the complete card path without a backend, API key, or microphone permission:

```text
https://localhost:5173/?mock=1
```

Mock mode automatically plays two acts. The original basic-catalog sequence covers
transcripts, task updates, loading placeholders (one replaced by its card and one
removed after failure), two card creations, a data-model patch, and a removal. A
second extended-catalog act adds a live Calgary forecast, a weekend comparison, and a
UV gauge, then patches the forecast chart and headline temperature in place. The
basic source messages are in `src/fixtures/mock-a2ui.jsonl`; the extended act and
pending-card frames are in `src/mock.ts`.

## UI

The dashboard uses a Gemini-inspired (but Voice2Canvas-branded) light/dark presentation:
Google-style neutral surfaces, relaxed type, generous pill geometry, blue/violet
gradient accents, and chat-shaped transcripts. It defaults to your system color
scheme; use the sun/moon pill in the top-right corner to switch themes. The choice is
saved locally in the browser. Fonts use local system fallbacks only; no font assets
are fetched at runtime.

A2UI surfaces appear as softly rounded, responsive cards on the full-window canvas.
The conversation peek above the voice dock expands into the complete, auto-scrolling
transcript, with filled user bubbles and plain assistant response blocks. Task
progress appears temporarily as pill-shaped status toasts instead of occupying canvas space.
`card_pending` frames reserve a final canvas slot with a shimmer until the first A2UI
bootstrap replaces it; failed tasks remove their pending slot with the normal exit
transition. Each `ready` frame clears only server-owned cards and placeholders before
the dashboard replay, preserving local transcript and task-toast history.

The Architecture button beside the theme toggle opens a full-screen, theme-aware system
map of the browser, ADK live backend, worker graph, card registry, and external Gemini,
Search, and Open-Meteo services. It is scrollable on narrow screens and closes with
Escape or its close control.

Surfaces using `https://v2ui.local/catalogs/extended/v1` can compose all basic A2UI
components with the extended display set: `Stat`, `StatGroup`, `LineChart`,
`BarChart`, `Gauge`, `ProgressBar`, `KeyValueList`, `Badge`, and `DataTable`. Bindable
values and collections react to `updateDataModel` patches, including animated chart,
gauge, and progress changes. Unknown or malformed extended components render an
inline warning chip without taking down the rest of the card.

When a live voice session is active, press `Space` outside an input or editable field
to toggle microphone mute. The dock also provides explicit mute and stop controls.

## Checks

```bash
npm run typecheck
npm test
npm run build
```

## Browser requirements

Use a current browser with Web Audio `AudioWorklet` support and grant microphone
permission. `getUserMedia` only works in a secure context: Vite serves HTTPS by
default in development (`https://localhost:5173` and `https://<lan-ip>:5173`),
ensuring microphone capture functions on LAN devices without Chrome flags.
The mic path captures mono audio, downsamples it
to 16 kHz, encodes 16-bit little-endian PCM, and sends roughly 200 ms binary frames.
Playback schedules the server's 24 kHz little-endian PCM buffers and flushes the queue
on an `interrupted` frame.

## A2UI renderer

The client uses the official `@a2ui/react/v0_9` renderer with
`@a2ui/web_core/v0_9`'s `MessageProcessor`, encapsulated in `src/a2ui/renderer.tsx`.
The installed renderer's `basicCatalog` still exposes a v0.9 catalog URL, while this
project's protocol requires the v0.9.1 URL. The adapter builds an equivalent catalog
under the protocol URL, so valid backend `createSurface` messages are consumed as-is.
The extended catalog is registered alongside it and includes the same basic component
implementations. No fallback renderer is used.
