package server

import (
	"encoding/json"
	"net/http"
)

// The /api/agents endpoint describes the live agent roster, the tools each
// agent holds, and the data stores they reach — including whether each data
// source is actually configured in this process's environment. The roster
// mirrors the agents constructed in internal/orchestrator; keep the two in
// sync when adding an agent.

type agentTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type agentDataSource struct {
	Name       string `json:"name"`
	Detail     string `json:"detail"`
	Configured bool   `json:"configured"`
}

type agentInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Kind        string            `json:"kind"` // live | llm | system
	Model       string            `json:"model,omitempty"`
	Description string            `json:"description"`
	Tools       []agentTool       `json:"tools"`
	DataSources []agentDataSource `json:"dataSources"`
	VoiceBack   bool              `json:"voiceBack"`
}

type agentsResponse struct {
	Agents []agentInfo `json:"agents"`
}

func agentRoster(cfg Config) agentsResponse {
	registry := agentDataSource{Name: "Card registry", Detail: "process-global cards, data models, layout", Configured: true}
	gemini := agentDataSource{Name: "Gemini API", Detail: "model calls", Configured: cfg.Auth.AuthConfigured()}

	return agentsResponse{Agents: []agentInfo{
		{
			ID: "voice-front-door", Name: "Voice front door", Kind: "live", Model: cfg.LiveModel,
			Description: "Listens and speaks over the live session; extracts every actionable request and dispatches it without blocking the conversation.",
			Tools: []agentTool{
				{Name: "dispatch_task", Description: "fast-ack dispatch: create / update / remove / investigate / arrange, domain general|weather|markets, optional refreshSeconds"},
				{Name: "list_cards", Description: "instant registry lookup to resolve “that card” references"},
			},
			DataSources: []agentDataSource{
				{Name: "Gemini Live API", Detail: "WSS, live audio + text turns", Configured: cfg.Auth.AuthConfigured()},
				registry,
			},
			VoiceBack: true,
		},
		{
			ID: "researcher", Name: "Researcher", Kind: "llm", Model: cfg.CardModel,
			Description: "Gathers current news and general facts into a compact sourced data brief.",
			Tools: []agentTool{
				{Name: "google_search", Description: "Google Search grounding (built-in)"},
			},
			DataSources: []agentDataSource{
				{Name: "Google Search", Detail: "grounding", Configured: cfg.Auth.AuthConfigured()},
				gemini,
			},
		},
		{
			ID: "weather-specialist", Name: "Weather", Kind: "llm", Model: cfg.CardModel,
			Description: "Resolves locations and gathers current conditions, hourly series, and daily forecasts.",
			Tools: []agentTool{
				{Name: "http_get", Description: "HTTPS GET restricted to Open-Meteo weather and geocoding endpoints"},
			},
			DataSources: []agentDataSource{
				{Name: "Open-Meteo", Detail: "weather + geocoding", Configured: true},
				gemini,
			},
		},
		{
			ID: "market-specialist", Name: "Markets", Kind: "llm", Model: cfg.CardModel,
			Description: "Builds sourced snapshots and comparisons for stocks, indexes, funds, crypto, currencies, and commodities.",
			Tools: []agentTool{
				{Name: "google_search", Description: "Google Search grounding for current public market data"},
			},
			DataSources: []agentDataSource{
				{Name: "Google Search", Detail: "public market sources; availability and quote delay vary", Configured: cfg.Auth.AuthConfigured()},
				gemini,
			},
		},
		{
			ID: "card-generator", Name: "Card generator", Kind: "llm", Model: cfg.CardModel,
			Description: "Turns a data brief into a titled A2UI extended-catalog card; schema-validated with one repair pass and a visible error-card fallback.",
			Tools: []agentTool{
				{Name: "http_get", Description: "HTTPS GET restricted to Open-Meteo hosts (weather + geocoding)"},
			},
			DataSources: []agentDataSource{
				{Name: "Open-Meteo", Detail: "weather + geocoding", Configured: true},
				registry, gemini,
			},
		},
		{
			ID: "investigator", Name: "Investigator", Kind: "llm", Model: cfg.CardModel,
			Description: "Digs into “explain / why” questions and returns a short finding that Gemini speaks aloud.",
			Tools: []agentTool{
				{Name: "google_search", Description: "Google Search grounding (built-in)"},
				{Name: "http_get", Description: "Open-Meteo HTTPS GET"},
			},
			DataSources: []agentDataSource{
				{Name: "Google Search", Detail: "grounding", Configured: cfg.Auth.AuthConfigured()},
				{Name: "Card data models", Detail: "current card state injected into prompts", Configured: true},
				gemini,
			},
			VoiceBack: true,
		},
		{
			ID: "layout-curator", Name: "Layout curator", Kind: "llm", Model: cfg.CardModel,
			Description: "Arranges the canvas: strict JSON card order and column spans, confirmed aloud through the live session.",
			Tools:       []agentTool{},
			DataSources: []agentDataSource{registry, gemini},
			VoiceBack:   true,
		},
		{
			ID: "auto-refresh", Name: "Auto-refresh", Kind: "system",
			Description: "Per-card tickers re-run the update pipeline on a requested interval and stream narrow data patches (max 4 refreshing cards).",
			Tools:       []agentTool{},
			DataSources: []agentDataSource{registry},
		},
	}}
}

func (h *handler) agentsAPI(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(agentRoster(h.cfg))
}
