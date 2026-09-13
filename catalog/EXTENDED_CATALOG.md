# Voice2Canvas Extended Card Catalog (v1)

Catalog ID: `https://v2ui.local/catalogs/extended/v1`

Extends the A2UI v0.9.1 **basic catalog** — every basic component (Card, Text, Image,
Icon, Row, Column, List, Tabs, Divider, Modal, Button, TextField, CheckBox,
ChoicePicker, Slider, DateTimeInput) remains valid and unchanged. This catalog adds
data-display components so cards can be genuinely rich. Machine-readable schema:
`catalog/extended-v1.schema.json` (source of truth; this doc is the narrative).

General rules (same as basic catalog):
- Flat adjacency list, exactly one `id: "root"` per surface, children by ID reference.
- Any scalar prop may be a literal or a `{ "path": "/json/pointer" }` data-model
  binding. Array-valued props (`points`, `values`, `rows`, `series`) may likewise be a
  literal array or a `{path}` binding to an array in the data model. **Prefer bindings
  for anything that may be updated later** — `updateDataModel` patches then animate
  charts in place.
- Numbers are raw (unformatted); `unit` / `format` props control display.

## Components

### `Stat`
Big-number metric. Props: `label` (string), `value` (number|string), `unit` (string?),
`delta` (number?, signed change), `deltaLabel` (string?, e.g. "vs yesterday"),
`tone` (`"neutral"|"positive"|"negative"|"warning"`, default neutral),
`spark` (array of numbers?, optional inline sparkline).

### `StatGroup`
Row/grid of Stats. Props: `children` (array of Stat component IDs), `columns`
(int?, default = count, max 4).

### `LineChart`
Trend chart. Props: `series` — array of `{ "name": string, "points": [{"x": string|number,
"y": number}] }` (or `{path}`); `unit` (string?), `height` (int?, px, default 180),
`yMin`/`yMax` (number?, autoscale when omitted), `fill` (bool?, area fill, default
false), `xType` (`"category"|"time"`, default category — when `"time"`, x values are
ISO 8601 strings).

### `BarChart`
Comparison chart. Props: `categories` (array of strings or `{path}`), `series` — array
of `{ "name": string, "values": [number] }` (or `{path}`); `unit` (string?),
`height` (int?, default 180), `stacked` (bool?, default false),
`horizontal` (bool?, default false).

### `Gauge`
Single bounded value. Props: `label` (string), `value` (number), `min` (number,
default 0), `max` (number), `unit` (string?), `thresholds` (array of
`{ "upTo": number, "tone": "positive"|"warning"|"negative" }`?, ordered ascending).

### `ProgressBar`
Props: `label` (string?), `value` (number, 0–100), `tone` (as Stat.tone?).

### `KeyValueList`
Aligned label/value rows. Props: `rows` — array of `{ "label": string, "value":
string|number, "tone": Stat.tone? }` (or `{path}`).

### `Badge`
Small semantic chip. Props: `text` (string), `tone` (as Stat.tone, default neutral).

### `DataTable`
Compact table. Props: `columns` (array of strings), `rows` (array of arrays of
string|number, or `{path}`), `align` (array of `"left"|"right"`?, per column).

## Authoring guidance for the card generator

Pick the card genre from the task:
- Current single values ("weather now", "price of X") → `Stat`/`StatGroup` + `Badge`
  (+ small `KeyValueList` for secondary facts).
- Trends / forecasts / history ("this week", "next 24 hours", "over time") →
  `LineChart` with a `Stat` headline.
- Comparisons across items ("compare X and Y", "top 5") → `BarChart` or `DataTable`.
- Single bounded metric (percentage, score, capacity) → `Gauge` or `ProgressBar`.
- Facts/definitions/lists → `KeyValueList` / `DataTable` / basic Text.
Compose: headline `Stat` row on top, chart or table below, source/footnote `Text` with
tone-muted styling at the bottom. Titles are short; put units in `unit` props, not in
labels.

## Surface bootstrap

`createSurface.catalogId` MUST be `https://v2ui.local/catalogs/extended/v1` for
new cards. The renderer keeps supporting basic-catalog surfaces (mock fixtures).
