package orchestrator

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"voice2canvas/backend/internal/registry"
)

const cardAgentInstruction = `You are the card-generation agent for a voice-driven test dashboard.

Turn the supplied task into one rich dashboard card using the Voice2Canvas extended catalog v1. Return one JSON object only. A full create envelope is:
{"title":"short registry title","description":"short registry description","components":[{"id":"root","component":"Card","child":"content"}],"dataModel":{}}
For an update, you may omit unchanged title, description, components, and dataModel and return narrow patches:
{"dataModelPatches":[{"path":"/forecast/series","value":[...]}]}
Patch paths are non-root JSON Pointer paths. Use dataModel only when creating a card or intentionally replacing the entire model. Never return Markdown fences, commentary, or an A2UI message list.

A2UI v0.9.1 + extended catalog authoring guide:
- The catalog ID is "https://voice2canvas.local/catalogs/extended/v1". It contains every basic component (Card, Text, Image, Icon, Row, Column, List, Tabs, Divider, Modal, Button, TextField, CheckBox, ChoicePicker, Slider, DateTimeInput) plus the extended components below.
- Components form a flat adjacency list. Every component has a unique id, child/children properties contain component IDs, and exactly one component has id "root". Usually use Card(root) -> Column(content).
- Any changing chart/stat value belongs in the surface dataModel. Bind it from a component with {"path":"/json/pointer"}; do not copy mutable values into component literals. Array-valued points, values, rows, and series can also be one {path} binding. Static labels and layout options may stay literal.

Create rules:
1. Title section: Every created card envelope must begin with a title. The first child of root must be a heading Text component with literal text that is short (5 words or fewer) and Title Case; set the envelope title to exactly the same text. "First child of root" means the first component rendered inside root: with the usual Card(root) -> Column(content) shape, put the heading first in content.children. Example: {"id":"root","component":"Card","child":"content"},{"id":"content","component":"Column","children":["title","body"]},{"id":"title","component":"Text","text":"Denver Weather","variant":"h3"}.

Extended component reference:
- Stat: label string; value number|string; optional unit string, delta number, deltaLabel string, tone neutral|positive|negative|warning, spark number[]. label/value/unit/delta/deltaLabel/spark accept {path} where their schema permits it.
- StatGroup: children is one or more Stat IDs; optional columns integer 1..4.
- LineChart: series is [{"name":string,"points":[{"x":string|number,"y":number}]}] or {path}; optional unit, height 80..480, yMin, yMax, fill, xType category|time. For time, x values are ISO 8601 strings.
- BarChart: categories is string[] or {path}; series is [{"name":string,"values":number[]}] or {path}; optional unit, height 80..480, stacked, horizontal.
- Gauge: label and value; required max; optional min, unit, and ascending thresholds [{"upTo":number,"tone":"positive"|"warning"|"negative"}]. label/value/unit accept {path} where allowed.
- ProgressBar: value is 0..100; optional label and tone neutral|positive|negative|warning. label/value accept {path}.
- KeyValueList: rows is [{"label":string,"value":string|number,"tone"?:neutral|positive|negative|warning}] or {path}.
- Badge: text plus optional tone neutral|positive|negative|warning. text accepts {path}.
- DataTable: columns is a nonempty string[]; rows is an array of string/number arrays or {path}; optional align entries left|right.

Choose the genre from the task:
- Current values -> Stat or StatGroup, optionally Badge and a small KeyValueList for secondary facts.
- Trends, forecasts, or history -> LineChart with a headline Stat.
- Comparisons across items -> BarChart or DataTable.
- A single bounded metric such as a percentage, score, or capacity -> Gauge or ProgressBar.
- Facts, definitions, and lists -> KeyValueList, DataTable, or basic Text.
- Market snapshots -> StatGroup for headline price and change, plus a LineChart for history or DataTable for comparisons.
- Weather conditions -> Stat or StatGroup for current values, plus a LineChart for forecasts and a Badge for conditions or alerts.
- News and general research -> KeyValueList, DataTable, or concise Text with clearly named sources.

Compose a compact card with the required title Text first, a headline Stat row next, a chart or table body after that, and a muted source footnote last (a Text with variant "caption"). Keep titles short and units in unit properties rather than labels.

Few-shot create — current conditions:
{"title":"Denver Now","description":"Current weather","dataModel":{"temperature":18,"condition":"Clear","facts":[{"label":"Humidity","value":"34%"},{"label":"Wind","value":"11 km/h"}]},"components":[{"id":"root","component":"Card","child":"content"},{"id":"content","component":"Column","children":["title","headline","condition","facts","source"]},{"id":"title","component":"Text","text":"Denver Now","variant":"h3"},{"id":"headline","component":"Stat","label":"Temperature","value":{"path":"/temperature"},"unit":"°C"},{"id":"condition","component":"Badge","text":{"path":"/condition"},"tone":"positive"},{"id":"facts","component":"KeyValueList","rows":{"path":"/facts"}},{"id":"source","component":"Text","text":"Source: Open-Meteo","variant":"caption"}]}

Few-shot create — bound forecast LineChart:
{"title":"Denver Forecast","description":"Next 12 hours","dataModel":{"current":{"temperature":18},"forecastSeries":[{"name":"Temperature","points":[{"x":"2026-08-02T00:00:00Z","y":18},{"x":"2026-08-02T03:00:00Z","y":16}]}]},"components":[{"id":"root","component":"Card","child":"content"},{"id":"content","component":"Column","children":["title","headline","chart","source"]},{"id":"title","component":"Text","text":"Denver Forecast","variant":"h3"},{"id":"headline","component":"Stat","label":"Current temperature","value":{"path":"/current/temperature"},"unit":"°C"},{"id":"chart","component":"LineChart","series":{"path":"/forecastSeries"},"unit":"°C","height":180,"xType":"time","fill":true},{"id":"source","component":"Text","text":"Source: Open-Meteo","variant":"caption"}]}

Few-shot create — comparison:
{"title":"Quarterly Revenue","description":"Regional comparison","dataModel":{"categories":["West","Central","East"],"series":[{"name":"Revenue","values":[42,35,51]}]},"components":[{"id":"root","component":"Card","child":"content"},{"id":"content","component":"Column","children":["title","chart","source"]},{"id":"title","component":"Text","text":"Quarterly Revenue","variant":"h3"},{"id":"chart","component":"BarChart","categories":{"path":"/categories"},"series":{"path":"/series"},"unit":"$M","height":180},{"id":"source","component":"Text","text":"sample data","variant":"caption"}]}

Update rules:
1. Title section: Every updated card envelope must still begin with a title. The first child of root must be a heading Text component with literal text that is short (5 words or fewer) and Title Case, and the registry title comes from that exact heading. "First child of root" means the first component rendered inside root, including the first item of a Card root's content Column. Preserve a compliant existing heading for data-only updates; if it is absent or the title must change, return replacement components with the heading first. Example: content.children begins ["title","body"] and title is {"id":"title","component":"Text","text":"Fleet Health","variant":"h3"}.
2. When intent is "update", inspect the supplied existing envelope and data model. If its component tree already fits the request and already has the required heading, preserve it and emit only the smallest dataModelPatches needed; this lets bound charts and stats update in place.
3. Return components only if the requested structure or genre must change or the required heading is missing. Return dataModel only if a whole-model replacement is truly necessary. Metadata may be omitted when unchanged.

Data rules:
- You have exactly one data tool, http_get, restricted to HTTPS api.open-meteo.com and geocoding-api.open-meteo.com.
- For weather forecasts and trends, fetch a real series. Geocode place names when necessary, then call Open-Meteo with fields such as hourly=temperature_2m or daily=temperature_2m_max,temperature_2m_min.
- Downsample every series to at most 24 points before storing it in dataModel. Keep matching timestamps/categories and values aligned.
- When the task includes a RESEARCH DATA BRIEF, treat its labeled facts, figures, units, timestamps, and source names as authoritative. Prefer brief data over fabrication and cite the brief's source names in the final muted caption Text.
- For any non-weather source absent from a research brief, fabricate plausible sample data and include a final caption Text whose exact text contains "sample data". Never fail or omit a requested card merely because live data is unavailable.`

const researcherAgentInstruction = `You are the research agent for a voice-driven dashboard card pipeline.
Given one dashboard-card task, use Google Search grounding to gather the concrete data the card needs: current figures, a short time series when trends are implied, entity names, units, and source names. Return a compact plain-text data brief of at most about 200 words. Use labeled facts and figures, include an "As of" timestamp, and name the sources. Do not return JSON or Markdown tables. If the task is weather-only, do not search; say "Weather-only: defer to Open-Meteo." Never invent facts.`

const weatherAgentInstruction = `You are the weather specialist for a general-purpose voice-driven dashboard.
Use http_get only with Open-Meteo weather and geocoding endpoints. Resolve place names before requesting conditions or forecasts. Gather the smallest useful set of current, hourly, or daily fields needed for the request, keep at most 24 representative points, preserve units and ISO timestamps, and return a compact plain-text data brief. Include the resolved location and an "As of" timestamp. Never invent observations, forecasts, or alerts. Do not return JSON or a Markdown table.`

const marketAgentInstruction = `You are the market-data specialist for a general-purpose voice-driven dashboard.
Use Google Search grounding to find current public market information for the requested ticker, asset, index, sector, currency, or commodity. Return a compact plain-text brief with the exact symbol or instrument, headline value, absolute and percentage change when available, currency, market/session status, a short time series or comparison when requested, an "As of" timestamp with timezone, and named sources. Clearly distinguish delayed quotes, previous closes, estimates, and confirmed results. Never provide personalized investment advice and never invent prices. Do not return JSON or a Markdown table.`

const investigatorAgentInstruction = `You are the investigator for a voice-driven dashboard.
Investigate the user's question using the supplied current card inventory and target card data. Use Google Search grounding for fresh external facts and http_get for fresh Open-Meteo weather or geocoding data when useful. When a specialist data brief is supplied, treat its sourced observations as authoritative and compose the spoken finding from it. Return only a finding of two to four spoken-friendly sentences. Do not use Markdown, headings, bullets, numbered lists, or JSON. Be concise, concrete, and conversational because another voice agent will read the finding aloud.`

const curatorAgentInstruction = `You are the layout curator for a voice-driven dashboard.
Use the supplied user instruction and complete live-card inventory to return exactly one strict JSON object, with no Markdown or commentary:
{"slots":[{"surfaceId":"card_1","order":1,"span":1}],"note":"Done — the requested card is up top."}
Cover every live card exactly once. Every order must be a unique positive integer and every span must be 1 or 2. Prefer span 2 for charts and tables when there are few cards, span 1 for stats, keep related cards adjacent, and put requested emphasis first. The note must be one short spoken sentence confirming what changed.`

// DataModelPatch becomes one narrow updateDataModel message. Value is always
// present in generated JSON; null is a valid replacement value.
type DataModelPatch struct {
	Path  string `json:"path"`
	Value any    `json:"value"`
}

// CardEnvelope carries the complete state for creates and either replacement
// state or narrow data-model patches for updates.
type CardEnvelope struct {
	Title            string           `json:"title,omitempty"`
	Description      string           `json:"description,omitempty"`
	Components       []map[string]any `json:"components,omitempty"`
	DataModel        map[string]any   `json:"dataModel,omitempty"`
	DataModelPatches []DataModelPatch `json:"dataModelPatches,omitempty"`
}

func cardPrompt(task Task, existing *CardEnvelope, dataBrief string) string {
	prompt := fmt.Sprintf("Task intent: %s\nTask description: %s", task.Intent, task.Description)
	if task.TargetCard != "" {
		prompt += "\nTarget card: " + task.TargetCard
	}
	if existing != nil {
		data, _ := json.Marshal(existing)
		prompt += "\nExisting extended-card envelope and current data model (preserve unchanged components and prefer narrow dataModelPatches): " + string(data)
	}
	if brief := strings.TrimSpace(dataBrief); brief != "" {
		prompt += "\nRESEARCH DATA BRIEF (authoritative; prefer these facts over fabrication and cite its source names in the card's muted footnote Text):\n" + brief
	}
	prompt += "\nReturn one JSON card envelope now."
	return prompt
}

func researchPrompt(task Task, existing *CardEnvelope) string {
	prompt := fmt.Sprintf("Dashboard-card task intent: %s\nTask description: %s", task.Intent, task.Description)
	if task.TargetCard != "" {
		prompt += "\nTarget card: " + task.TargetCard
	}
	if existing != nil {
		data, _ := json.Marshal(existing)
		prompt += "\nExisting card context: " + string(data)
	}
	return prompt + "\nReturn the compact plain-text data brief now."
}

func specialistPrompt(specialty string, task Task, existing *CardEnvelope) string {
	prompt := fmt.Sprintf("%s task intent: %s\nUser request: %s", specialty, task.Intent, task.Description)
	if task.TargetCard != "" {
		prompt += "\nTarget card: " + task.TargetCard
	}
	if existing != nil {
		data, _ := json.Marshal(existing)
		prompt += "\nExisting card context: " + string(data)
	}
	return prompt + "\nUse the specialist's configured public data tools and return the compact plain-text data brief now."
}

func investigationPrompt(task Task, cards []registry.Card, dataBrief string) string {
	type inventoryCard struct {
		SurfaceID   string `json:"surfaceId"`
		Title       string `json:"title"`
		Description string `json:"description"`
		CreatedAt   string `json:"createdAt"`
		Order       int    `json:"order"`
	}
	inventory := make([]inventoryCard, 0, len(cards))
	var target any
	for _, card := range cards {
		inventory = append(inventory, inventoryCard{
			SurfaceID: card.SurfaceID, Title: card.Title, Description: card.Description,
			CreatedAt: card.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z07:00"), Order: card.Order,
		})
		if task.TargetCard != "" && card.SurfaceID == task.TargetCard {
			target = map[string]any{
				"surfaceId":   card.SurfaceID,
				"title":       card.Title,
				"description": card.Description,
				"components":  card.Components,
				"dataModel":   card.DataModelSummary,
				"order":       card.Order,
			}
		}
	}
	inventoryJSON, _ := json.Marshal(inventory)
	targetJSON, _ := json.Marshal(target)
	prompt := fmt.Sprintf("Investigation question: %s\nRequested target card: %s\nCurrent card inventory: %s\nTarget card full rendered state: %s",
		task.Description, task.TargetCard, inventoryJSON, targetJSON)
	if brief := strings.TrimSpace(dataBrief); brief != "" {
		prompt += "\nSPECIALIST DATA BRIEF (authoritative):\n" + brief
	}
	return prompt + "\nReturn the spoken finding now."
}

type curatedLayout struct {
	Slots []registry.Slot `json:"slots"`
	Note  string          `json:"note"`
}

func curatorPrompt(task Task, cards []registry.Card) string {
	type inventoryCard struct {
		SurfaceID      string   `json:"surfaceId"`
		Title          string   `json:"title"`
		Description    string   `json:"description"`
		ComponentKinds []string `json:"componentKinds"`
		Order          int      `json:"order"`
		Span           int      `json:"span"`
	}
	inventory := make([]inventoryCard, 0, len(cards))
	for _, card := range cards {
		inventory = append(inventory, inventoryCard{
			SurfaceID: card.SurfaceID, Title: card.Title, Description: card.Description,
			ComponentKinds: componentKinds(card.Components), Order: card.Order, Span: card.Span,
		})
	}
	inventoryJSON, _ := json.Marshal(inventory)
	return fmt.Sprintf("User layout instruction: %s\nTarget card, if any: %s\nComplete live-card inventory: %s\nReturn the strict JSON layout now.",
		task.Description, task.TargetCard, inventoryJSON)
}

func componentKinds(components []map[string]any) []string {
	seen := make(map[string]bool)
	for _, component := range components {
		name, _ := component["component"].(string)
		var kind string
		switch name {
		case "LineChart", "BarChart":
			kind = "chart"
		case "DataTable", "KeyValueList":
			kind = "table"
		case "Stat", "StatGroup", "Gauge", "ProgressBar":
			kind = "stat"
		}
		if kind != "" {
			seen[kind] = true
		}
	}
	result := make([]string, 0, 3)
	for _, kind := range []string{"chart", "table", "stat"} {
		if seen[kind] {
			result = append(result, kind)
		}
	}
	return result
}

func parseCuratedLayout(raw string, cards []registry.Card) (curatedLayout, error) {
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(raw)))
	decoder.DisallowUnknownFields()
	var layout curatedLayout
	if err := decoder.Decode(&layout); err != nil {
		return curatedLayout{}, fmt.Errorf("parse curator JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return curatedLayout{}, fmt.Errorf("parse curator JSON: multiple values")
		}
		return curatedLayout{}, fmt.Errorf("parse curator JSON trailing content: %w", err)
	}
	layout.Note = strings.TrimSpace(layout.Note)
	if layout.Note == "" {
		return curatedLayout{}, fmt.Errorf("curator note is required")
	}
	if len(layout.Slots) != len(cards) {
		return curatedLayout{}, fmt.Errorf("curator returned %d slots for %d live cards", len(layout.Slots), len(cards))
	}
	live := make(map[string]struct{}, len(cards))
	for _, card := range cards {
		live[card.SurfaceID] = struct{}{}
	}
	seenCards := make(map[string]struct{}, len(cards))
	seenOrders := make(map[int]struct{}, len(cards))
	for index, slot := range layout.Slots {
		if _, ok := live[slot.SurfaceID]; !ok {
			return curatedLayout{}, fmt.Errorf("slots[%d] has unknown surfaceId %q", index, slot.SurfaceID)
		}
		if _, duplicate := seenCards[slot.SurfaceID]; duplicate {
			return curatedLayout{}, fmt.Errorf("slots[%d] repeats surfaceId %q", index, slot.SurfaceID)
		}
		if slot.Order <= 0 {
			return curatedLayout{}, fmt.Errorf("slots[%d].order must be positive", index)
		}
		if _, duplicate := seenOrders[slot.Order]; duplicate {
			return curatedLayout{}, fmt.Errorf("slots[%d] repeats order %d", index, slot.Order)
		}
		if slot.Span != 1 && slot.Span != 2 {
			return curatedLayout{}, fmt.Errorf("slots[%d].span must be 1 or 2", index)
		}
		seenCards[slot.SurfaceID] = struct{}{}
		seenOrders[slot.Order] = struct{}{}
	}
	return layout, nil
}

func buildCuratorRepairPrompt(raw string, validationErr error, cards []registry.Card) string {
	current := make([]registry.Slot, 0, len(cards))
	for _, card := range cards {
		current = append(current, registry.Slot{SurfaceID: card.SurfaceID, Order: card.Order, Span: card.Span})
	}
	currentJSON, _ := json.Marshal(current)
	return fmt.Sprintf("The previous curator output was invalid.\nValidation error: %s\nPrevious output: %s\nRequired live cards and current fallback placement: %s\nRepair it. Return one strict JSON object only with slots covering every card exactly once and a one-sentence note.",
		validationErrorText(validationErr), strings.TrimSpace(raw), currentJSON)
}

// BuildRepairPrompt is exported so the validation/repair contract can be
// tested independently of a live Gemini call.
func BuildRepairPrompt(rawOutput string, validationErr error) string {
	return fmt.Sprintf(`The previous extended-card generation output was invalid.

Validation errors (preserve these verbatim while diagnosing the repair):
%s

Previous output:
%s

Repair the output according to the extended catalog authoring guide. Return one JSON object only. A create needs title, description, components, and dataModel; an update may instead use dataModelPatches with non-root JSON Pointer paths. Every card must begin with a literal heading Text as the first child of root (the first component rendered inside a Card root's content Column); keep it to 5 words or fewer in Title Case and set the envelope title to exactly that heading. Do not return Markdown fences, an A2UI message list, commentary, or explanatory text.`, validationErrorText(validationErr), strings.TrimSpace(rawOutput))
}

func validationErrorText(err error) string {
	if err == nil {
		return "unknown validation error"
	}
	return strings.TrimSpace(err.Error())
}

func parseEnvelope(raw string) (CardEnvelope, error) {
	cleaned := stripJSONFences(raw)
	var envelope CardEnvelope
	if err := json.Unmarshal([]byte(cleaned), &envelope); err != nil {
		return CardEnvelope{}, fmt.Errorf("parse card envelope JSON: %w", err)
	}
	envelope.Title = strings.TrimSpace(envelope.Title)
	envelope.Description = strings.TrimSpace(envelope.Description)
	if envelope.Title == "" && envelope.Description == "" && envelope.Components == nil && envelope.DataModel == nil && len(envelope.DataModelPatches) == 0 {
		return CardEnvelope{}, fmt.Errorf("card envelope contains no changes")
	}
	for index := range envelope.DataModelPatches {
		envelope.DataModelPatches[index].Path = strings.TrimSpace(envelope.DataModelPatches[index].Path)
		if !strings.HasPrefix(envelope.DataModelPatches[index].Path, "/") || envelope.DataModelPatches[index].Path == "/" {
			return CardEnvelope{}, fmt.Errorf("dataModelPatches[%d].path must be a non-root JSON Pointer", index)
		}
	}
	return envelope, nil
}

func stripJSONFences(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		if newline := strings.IndexByte(trimmed, '\n'); newline >= 0 {
			trimmed = trimmed[newline+1:]
		}
		trimmed = strings.TrimSuffix(strings.TrimSpace(trimmed), "```")
	}
	if start := strings.IndexByte(trimmed, '{'); start >= 0 {
		if end := strings.LastIndexByte(trimmed, '}'); end >= start {
			return strings.TrimSpace(trimmed[start : end+1])
		}
	}
	return trimmed
}
