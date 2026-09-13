# Voice2Canvas WebSocket Protocol (v0 — test build)

Single WebSocket between browser and Go server: `ws://<host>:8080/ws`.

## Frame types

- **Binary frames (client → server):** raw audio chunks — 16-bit little-endian PCM,
  16 kHz, mono. No wrapper. Server forwards them into the Gemini Live session.
- **Binary frames (server → client):** raw audio chunks — 16-bit little-endian PCM,
  24 kHz, mono (Gemini's voice output). Client plays them immediately; on
  `interrupted` (below) the client must flush its playback queue.
- **Text frames (both directions):** single JSON object per frame.

## JSON messages: server → client

| `type` | Fields | Meaning |
|---|---|---|
| `ready` | `sessionId` | Live session established; start sending audio |
| `input_transcript` | `text`, `final` (bool) | ASR of the user's speech |
| `output_transcript` | `text`, `final` (bool) | Transcript of Gemini's spoken reply |
| `interrupted` | — | User barged in; flush queued playback audio |
| `a2ui` | `surfaceId`, `messages` (array of A2UI v0.9.1 message objects) | Render/update a card. Client feeds each element of `messages`, in order, to the A2UI MessageProcessor |
| `card_removed` | `surfaceId` | Card deleted (server already sent `deleteSurface` in a prior `a2ui` frame; this is the app-level signal to drop the slot) |
| `layout` | `slots`: array of `{surfaceId, order, span?}` | Display order of cards; optional `span` (1\|2, default 1) makes a card occupy two grid columns |
| `task_status` | `taskId`, `surfaceId?`, `status` (`dispatched`\|`researching`\|`generating`\|`rendered`\|`failed`), `detail?` | Progress of a dispatched task; `researching` precedes `generating` when a research stage runs |
| `card_pending` | `surfaceId`, `taskId`, `title?` | A card slot was reserved and generation started; client shows a loading-shimmer placeholder in that slot until the surface's first `a2ui` frame arrives (which replaces it) or the task fails (which removes it) |
| `error` | `message`, `fatal` (bool) | Server-side error; if `fatal`, client shows it and stops audio |
| `usage` | `inputTokens` (int), `outputTokens` (int), `source` (`live`\|`worker`), `at` (ISO 8601) | Token usage of one completed model call (live-session turn or background agent call). Client aggregates rolling-window rates |

## JSON messages: client → server

| `type` | Fields | Meaning |
|---|---|---|
| `start` | `apiKey?` | Begin (or restart) a Live session; an optional non-blank Gemini key takes precedence over server auth for this connection |
| `audio_end` | — | Mic muted/stopped (server sends `audioStreamEnd` to Gemini) |
| `action` | `surfaceId`, `name`, `sourceComponentId`, `context` (object) | A2UI user action from a card, forwarded verbatim to the orchestrator |
| `stop` | — | Tear down the Live session (server keeps WS open) |

## Dashboard state replay (reconnect semantics)

Card state is **process-global on the server** (not per-connection). When a client
connects and sends `start`, the server — after `ready` — replays the current dashboard:
for every registered card, the full `a2ui` bootstrap frames (createSurface +
updateComponents + updateDataModel), then one `layout` frame. A page refresh therefore
restores the canvas. Clients must treat surfaces idempotently: an `a2ui` frame for an
already-known `surfaceId` re-bootstraps that surface. Voice/Live session state is NOT
replayed — only cards and layout.

## Investigations (non-card tasks)

`dispatch_task` accepts a fourth intent, `investigate`, for questions/analysis that
don't create or change cards ("explain this data", "why is X higher than Y"). The
orchestrator runs an analyst agent (with access to current card data models and the
HTTP data tool); on completion the server injects the finding as a text turn into the
live Gemini session, which voices it to the user ("Here's what I found: …"). The
finding also flows through `task_status` (`rendered` with `detail` = finding summary).
No new client frames are required; the spoken answer arrives via normal audio +
`output_transcript`.

## A2UI payload rules

- Spec **v0.9.1**, basic catalog
  (`catalogId: "https://a2ui.org/specification/v0_9_1/catalogs/basic/catalog.json"`).
- One card = one `surfaceId` (server-generated, `card_<n>`).
- New card: `createSurface` then `updateComponents` (exactly one component with
  `id: "root"`), optional `updateDataModel`.
- Card update: prefer narrow `updateDataModel` patches; full `updateComponents`
  replace is acceptable in the test build.
- Server validates every message against the v0.9.1 JSON schema before sending;
  invalid generations must never reach the client.

## Environment

Server reads an optional `apiKey` from the `start` frame, then falls back to
`GEMINI_API_KEY` or Google ADC. Reference clients must not persist or log the
browser-provided key. If no authentication is present the server must start,
serve health checks, and return a clear fatal `error` frame when `start` is
received — never crash at boot.

Live model: `gemini-3.1-flash-live-preview`. Card model: `gemini-3.6-flash`.
Both must be overridable via env (`V2UI_LIVE_MODEL`, `V2UI_CARD_MODEL`).
