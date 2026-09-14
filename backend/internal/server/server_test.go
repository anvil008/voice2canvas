package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/genai"

	"voice2canvas/backend/internal/a2ui"
	"voice2canvas/backend/internal/authconfig"
	"voice2canvas/backend/internal/registry"
)

// newTestClient builds a client the same way the websocket path does, so the
// speech queue is wired without needing a real connection.
func newTestClient(h *handler, hook func(any)) *client {
	c := newClient(nil, h)
	c.sendJSONHook = hook
	return c
}

func TestHealthzWithoutCredentials(t *testing.T) {
	handler, err := NewHandler(Config{Auth: authconfig.Config{}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	response, err := server.Client().Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body struct {
		OK             bool   `json:"ok"`
		LiveModel      string `json:"liveModel"`
		CardModel      string `json:"cardModel"`
		AuthConfigured bool   `json:"authConfigured"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.OK || body.LiveModel != DefaultLiveModel || body.CardModel != DefaultCardModel || body.AuthConfigured {
		t.Fatalf("unexpected health response: %+v", body)
	}
}

func TestStartWithoutCredentialsSendsFatalError(t *testing.T) {
	handler, err := NewHandler(Config{Auth: authconfig.Config{}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	connection, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := connection.WriteJSON(map[string]string{"type": "start"}); err != nil {
		t.Fatal(err)
	}
	if err := connection.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, payload, err := connection.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var frame struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Fatal   bool   `json:"fatal"`
	}
	if err := json.Unmarshal(payload, &frame); err != nil {
		t.Fatal(err)
	}
	if frame.Type != "error" || !frame.Fatal || !strings.Contains(frame.Message, "GEMINI_API_KEY") {
		t.Fatalf("unexpected no-auth start frame: %+v", frame)
	}
}

type capturedFrames struct {
	mu     sync.Mutex
	frames []map[string]any
}

func (c *capturedFrames) add(value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	frame, ok := value.(map[string]any)
	if !ok {
		return
	}
	c.frames = append(c.frames, frame)
}

func (c *capturedFrames) snapshot() []map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]map[string]any(nil), c.frames...)
}

func newStateTestHandler(t *testing.T) *handler {
	t.Helper()
	validator, err := a2ui.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	return &handler{
		validator: validator,
		registry:  registry.New(),
		clients:   make(map[*client]struct{}),
	}
}

func TestReplayDashboardAfterSimulatedReconnect(t *testing.T) {
	h := newStateTestHandler(t)
	reserved := h.registry.Reserve()
	h.registry.CommitState(reserved, "Weather", "Current conditions", []map[string]any{
		{"id": "root", "component": "Card", "child": "value"},
		{"id": "value", "component": "Stat", "label": "Temperature", "value": map[string]any{"path": "/temperature"}},
	}, map[string]any{"temperature": 18.0, "condition": "Clear"})
	// Simulate the effective state after a narrow updateDataModel patch was
	// folded by the orchestrator before the old connection disappeared.
	h.registry.Upsert(reserved.SurfaceID, "", "", map[string]any{"temperature": 22.5, "condition": "Clear"})
	if err := h.registry.ApplyLayout([]registry.Slot{{SurfaceID: reserved.SurfaceID, Order: 1, Span: 2}}); err != nil {
		t.Fatal(err)
	}

	capture := &capturedFrames{}
	reconnected := newTestClient(h, capture.add)
	h.replayDashboard(reconnected)
	frames := capture.snapshot()
	if len(frames) != 2 || frames[0]["type"] != "a2ui" || frames[1]["type"] != "layout" {
		t.Fatalf("unexpected replay frames: %#v", frames)
	}
	messages := frames[0]["messages"].([]map[string]any)
	if len(messages) != 3 {
		t.Fatalf("replay did not emit a full bootstrap: %#v", messages)
	}
	update := messages[2]["updateDataModel"].(map[string]any)
	model := update["value"].(map[string]any)
	if model["temperature"] != 22.5 {
		t.Fatalf("replay used stale data model: %#v", model)
	}
	slots := frames[1]["slots"].([]registry.Slot)
	if len(slots) != 1 || slots[0].SurfaceID != reserved.SurfaceID || slots[0].Span != 2 {
		t.Fatalf("replay lost curated span: %+v", slots)
	}
}

func TestBroadcastJSONReachesTwoConnectedClients(t *testing.T) {
	h := newStateTestHandler(t)
	firstCapture, secondCapture := &capturedFrames{}, &capturedFrames{}
	first := newTestClient(h, firstCapture.add)
	second := newTestClient(h, secondCapture.add)
	h.addClient(first)
	h.addClient(second)

	h.broadcastJSON(map[string]any{"type": "card_pending", "surfaceId": "card_1", "taskId": "task_1"})
	for name, capture := range map[string]*capturedFrames{"first": firstCapture, "second": secondCapture} {
		frames := capture.snapshot()
		if len(frames) != 1 || frames[0]["type"] != "card_pending" {
			t.Fatalf("%s client missed broadcast: %#v", name, frames)
		}
	}
}

func TestUsageBroadcastReachesAllConnectedClients(t *testing.T) {
	h := newStateTestHandler(t)
	firstCapture, secondCapture := &capturedFrames{}, &capturedFrames{}
	h.addClient(newTestClient(h, firstCapture.add))
	h.addClient(newTestClient(h, secondCapture.add))

	h.broadcastUsage(21, 8, "worker")
	for name, capture := range map[string]*capturedFrames{"first": firstCapture, "second": secondCapture} {
		frames := capture.snapshot()
		if len(frames) != 1 {
			t.Fatalf("%s client received unexpected usage frames: %#v", name, frames)
		}
		frame := frames[0]
		if frame["type"] != "usage" || frame["inputTokens"] != 21 || frame["outputTokens"] != 8 || frame["source"] != "worker" {
			t.Fatalf("%s client received malformed usage frame: %#v", name, frame)
		}
		at, ok := frame["at"].(string)
		if !ok {
			t.Fatalf("%s usage frame omitted timestamp: %#v", name, frame)
		}
		if _, err := time.Parse(time.RFC3339Nano, at); err != nil {
			t.Fatalf("%s usage frame has invalid timestamp %q: %v", name, at, err)
		}
	}
}

func TestListCardInventoryAndDispatchSchema(t *testing.T) {
	cardRegistry := registry.New()
	cardRegistry.Create("Weather", "Denver", map[string]any{"temperature": 20})
	cardRegistry.Create("Revenue", "Quarterly", map[string]any{"value": 42})
	items := listCardInventory(cardRegistry)
	if len(items) != 2 || items[0].SurfaceID != "card_1" || items[1].Order != 2 || items[0].CreatedAt == "" {
		t.Fatalf("unexpected list_cards inventory: %#v", items)
	}

	schema := dispatchTaskInputSchema()
	refresh := schema.Properties["refreshSeconds"]
	if refresh.Type != "integer" || refresh.Minimum == nil || *refresh.Minimum != 30 || refresh.Maximum == nil || *refresh.Maximum != 3600 {
		t.Fatalf("refreshSeconds bounds missing from dispatch schema: %#v", refresh)
	}
	intentJSON, _ := json.Marshal(schema.Properties["intent"].Enum)
	if !strings.Contains(string(intentJSON), "investigate") {
		t.Fatalf("investigate missing from intent schema: %s", intentJSON)
	}
	if !strings.Contains(string(intentJSON), "arrange") {
		t.Fatalf("arrange missing from intent schema: %s", intentJSON)
	}
	domainJSON, _ := json.Marshal(schema.Properties["domain"].Enum)
	for _, domain := range []string{"general", "weather", "markets"} {
		if !strings.Contains(string(domainJSON), domain) || !strings.Contains(liveAgentInstruction, `domain to "`+domain+`"`) && domain != "general" {
			t.Fatalf("domain routing guidance/schema missing %q: %s", domain, domainJSON)
		}
	}
	listTool, err := newListCardsTool(cardRegistry)
	if err != nil || listTool.Name() != "list_cards" {
		t.Fatalf("list_cards function tool was not created: %v, %#v", err, listTool)
	}
}

func TestAgentRosterUsesGeneralPurposeSpecialists(t *testing.T) {
	roster := agentRoster(Config{})
	ids := make(map[string]bool, len(roster.Agents))
	for _, item := range roster.Agents {
		ids[item.ID] = true
	}
	for _, expected := range []string{"weather-specialist", "market-specialist", "researcher", "card-generator"} {
		if !ids[expected] {
			t.Errorf("missing general-purpose agent %q", expected)
		}
	}
	for _, personal := range []string{"infra", "homelab", "proxmox"} {
		if ids[personal] {
			t.Errorf("personal agent %q remains in roster", personal)
		}
	}
}

type stubLiveSession struct {
	mu       sync.Mutex
	requests []agent.LiveRequest
	closed   bool
}

func (s *stubLiveSession) Send(request agent.LiveRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errLiveSessionUnavailable
	}
	s.requests = append(s.requests, request)
	return nil
}

func (s *stubLiveSession) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}

func TestInvestigationFindingInjectedAndClosedSessionDropped(t *testing.T) {
	live := &stubLiveSession{}
	c := newTestClient(nil, nil)
	c.live = live
	finding := "[investigation result — relay to the user conversationally, briefly] The increase came from wind."
	if err := c.deliverFinding(finding); err != nil {
		t.Fatal(err)
	}
	live.mu.Lock()
	if len(live.requests) != 1 || live.requests[0].Content == nil || live.requests[0].Content.Role != genai.RoleUser || live.requests[0].Content.Parts[0].Text != finding {
		t.Fatalf("unexpected injected live request: %#v", live.requests)
	}
	live.mu.Unlock()

	c.stopLive()
	if err := c.deliverFinding("late result"); err != nil {
		t.Fatalf("closed live session should drop finding safely: %v", err)
	}
}

func TestAuthForStartPrefersEphemeralBrowserKey(t *testing.T) {
	fallback := authconfig.Config{APIKey: "server-key", Project: "project", Location: "location"}
	got := authForStart(fallback, "  browser-key  ")
	if got.APIKey != "browser-key" || got.Project != "" || got.Location != "" {
		t.Fatalf("unexpected browser auth: %#v", got)
	}

	got = authForStart(fallback, "  ")
	if got != fallback {
		t.Fatalf("blank browser key should preserve server auth: %#v", got)
	}
}

func TestOriginChecking(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/ws", nil)
	req.Host = "example.com"

	req.Header.Del("Origin")
	if !isOriginAllowed(req) {
		t.Fatal("empty origin should be allowed")
	}

	req.Header.Set("Origin", "http://example.com")
	if !isOriginAllowed(req) {
		t.Fatal("same host origin should be allowed")
	}

	req.Header.Set("Origin", "http://evil.com")
	if isOriginAllowed(req) {
		t.Fatal("different origin should be rejected by default")
	}

	t.Setenv("VOICE2CANVAS_ALLOWED_ORIGINS", "http://evil.com, https://trusted.app")
	if !isOriginAllowed(req) {
		t.Fatal("origin in VOICE2CANVAS_ALLOWED_ORIGINS should be allowed")
	}
}

func TestDebugCardsGate(t *testing.T) {
	handler, err := NewHandler(Config{})
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("VOICE2CANVAS_DEBUG", "")
	t.Setenv("V2UI_DEBUG", "")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/debug/cards", nil))
	if rec.Code != 404 {
		t.Fatalf("expected 404 when debug is disabled, got %d", rec.Code)
	}

	t.Setenv("VOICE2CANVAS_DEBUG", "true")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/debug/cards", nil))
	if rec.Code != 200 {
		t.Fatalf("expected 200 when debug is enabled, got %d", rec.Code)
	}
}
