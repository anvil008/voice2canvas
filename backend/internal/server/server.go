// Package server exposes the HTTP health endpoint and the protocol WebSocket.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/genai"

	"voice2canvas/backend/internal/a2ui"
	"voice2canvas/backend/internal/authconfig"
	"voice2canvas/backend/internal/orchestrator"
	"voice2canvas/backend/internal/registry"
)

const (
	DefaultLiveModel  = "gemini-3.1-flash-live-preview"
	DefaultCardModel  = "gemini-3.6-flash"
	DefaultListenAddr = ":8080"
)

// Config controls server defaults and is intentionally small enough to use in
// tests without reading process environment variables.
type Config struct {
	LiveModel  string
	CardModel  string
	Auth       authconfig.Config
	Validator  *a2ui.Validator
	ListenAddr string
	StaticDir  string
}

func ConfigFromEnv() Config {
	liveModel := firstEnv("VOICE2CANVAS_LIVE_MODEL", "V2UI_LIVE_MODEL")
	if liveModel == "" {
		liveModel = DefaultLiveModel
	}
	cardModel := firstEnv("VOICE2CANVAS_CARD_MODEL", "V2UI_CARD_MODEL")
	if cardModel == "" {
		cardModel = DefaultCardModel
	}
	listenAddr := resolveListenAddr()
	staticDir := firstEnv("VOICE2CANVAS_STATIC_DIR", "V2UI_STATIC_DIR")
	return Config{
		LiveModel:  liveModel,
		CardModel:  cardModel,
		Auth:       authconfig.FromEnv(),
		ListenAddr: listenAddr,
		StaticDir:  staticDir,
	}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if val := strings.TrimSpace(os.Getenv(key)); val != "" {
			return val
		}
	}
	return ""
}

func resolveListenAddr() string {
	for _, key := range []string{"VOICE2CANVAS_LISTEN_ADDR", "V2UI_LISTEN_ADDR", "PORT"} {
		if val := strings.TrimSpace(os.Getenv(key)); val != "" {
			if !strings.Contains(val, ":") {
				return ":" + val
			}
			return val
		}
	}
	return DefaultListenAddr
}

// ValidateListenAddr normalizes the optional listener value and rejects values
// that are not valid host:port TCP addresses.
func ValidateListenAddr(address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		address = DefaultListenAddr
	}
	if strings.ContainsAny(address, " \t\r\n") {
		return "", fmt.Errorf("listen address must be a host:port address, got %q", address)
	}
	if !strings.Contains(address, ":") {
		address = ":" + address
	}
	_, port, err := net.SplitHostPort(address)
	if err != nil || port == "" {
		return "", fmt.Errorf("listen address must be a host:port address, got %q", address)
	}
	parsedPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil || parsedPort > 65535 {
		return "", fmt.Errorf("listen address has an invalid port, got %q", address)
	}
	return address, nil
}

type handler struct {
	cfg       Config
	validator *a2ui.Validator
	registry  *registry.Registry
	refreshes *orchestrator.RefreshCoordinator
	upgrader  websocket.Upgrader

	clientsMu   sync.RWMutex
	clients     map[*client]struct{}
	broadcastMu sync.Mutex
}

// NewHandler creates a handler without binding a port or requiring API
// credentials. Schema compilation is local and deterministic.
func NewHandler(cfg Config) (http.Handler, error) {
	if cfg.LiveModel == "" {
		cfg.LiveModel = DefaultLiveModel
	}
	if cfg.CardModel == "" {
		cfg.CardModel = DefaultCardModel
	}
	validator := cfg.Validator
	if validator == nil {
		var err error
		validator, err = a2ui.NewValidator()
		if err != nil {
			return nil, err
		}
	}
	staticDir, err := normalizeStaticDir(cfg.StaticDir)
	if err != nil {
		return nil, err
	}
	cfg.StaticDir = staticDir
	h := &handler{
		cfg:       cfg,
		validator: validator,
		registry:  registry.New(),
		refreshes: orchestrator.NewRefreshCoordinator(),
		clients:   make(map[*client]struct{}),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  16 * 1024,
			WriteBufferSize: 16 * 1024,
			CheckOrigin: func(_ *http.Request) bool {
				// This is a local test dashboard; deployments should put an
				// origin policy in front of it.
				return true
			},
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.healthz)
	mux.HandleFunc("/ws", h.websocket)
	mux.HandleFunc("/api/agents", h.agentsAPI)
	mux.HandleFunc("/debug/cards", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(listCardInventory(h.registry))
	})
	if cfg.StaticDir != "" {
		mux.Handle("/", newProductionStaticHandler(cfg.StaticDir))
	}
	return mux, nil
}

func (h *handler) healthz(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"ok":             true,
		"liveModel":      h.cfg.LiveModel,
		"cardModel":      h.cfg.CardModel,
		"authConfigured": h.cfg.Auth.AuthConfigured(),
	})
}

func (h *handler) websocket(writer http.ResponseWriter, request *http.Request) {
	connection, err := h.upgrader.Upgrade(writer, request, nil)
	if err != nil {
		return
	}
	client := newClient(connection, h)
	h.addClient(client)
	defer h.removeClient(client)
	defer client.close()
	client.readLoop()
}

type client struct {
	conn *websocket.Conn
	h    *handler

	writeMu    sync.Mutex
	stateMu    sync.Mutex
	liveSendMu sync.Mutex
	live       agent.LiveSession
	cancel     context.CancelFunc
	orch       *orchestrator.Orchestrator

	sendJSONHook func(any)
	speech       *speechQueue
}

func newClient(conn *websocket.Conn, h *handler) *client {
	c := &client{conn: conn, h: h}
	c.speech = newSpeechQueue(c.speakFinding, func(err error) {
		c.sendError("deliver agent finding: "+err.Error(), false)
	})
	return c
}

func (c *client) close() {
	c.stopLive()
	_ = c.conn.Close()
}

func (c *client) readLoop() {
	for {
		messageType, payload, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		switch messageType {
		case websocket.BinaryMessage:
			c.sendAudio(payload)
		case websocket.TextMessage:
			c.handleJSON(payload)
		}
	}
}

type incomingMessage struct {
	Type              string         `json:"type"`
	APIKey            string         `json:"apiKey,omitempty"`
	SurfaceID         string         `json:"surfaceId,omitempty"`
	Name              string         `json:"name,omitempty"`
	SourceComponentID string         `json:"sourceComponentId,omitempty"`
	Context           map[string]any `json:"context,omitempty"`
}

func (c *client) handleJSON(payload []byte) {
	var message incomingMessage
	if err := json.Unmarshal(payload, &message); err != nil {
		c.sendError(fmt.Sprintf("invalid JSON frame: %v", err), false)
		return
	}
	switch message.Type {
	case "start":
		c.startLive(message.APIKey)
	case "audio_end":
		c.endAudioActivity()
	case "action":
		c.handleAction(message)
	case "stop":
		c.stopLive()
	default:
		c.sendError("unknown client message type: "+message.Type, false)
	}
}

func (c *client) sendAudio(payload []byte) {
	// PROTOCOL.md defines 16 kHz, mono, signed 16-bit little-endian PCM.
	request := agent.LiveRequest{RealtimeInput: &genai.Blob{
		Data:     append([]byte(nil), payload...),
		MIMEType: "audio/pcm;rate=16000",
	}}
	if err := c.sendLiveRequest(request); err != nil {
		if errors.Is(err, errLiveSessionUnavailable) {
			c.sendError("audio received before a live session was started", false)
			return
		}
		c.sendError("send audio to Gemini: "+err.Error(), false)
	}
}

func (c *client) endAudioActivity() {
	// ADK v2.1.0's public LiveRequest accepts ActivityEnd but does not expose
	// genai.LiveRealtimeInput.AudioStreamEnd. This is the supported activity
	// boundary; automatic VAD remains enabled in the LiveRunConfig.
	if err := c.sendLiveRequest(agent.LiveRequest{RealtimeInput: &genai.ActivityEnd{}}); err != nil {
		if errors.Is(err, errLiveSessionUnavailable) {
			return
		}
		c.sendError("end Gemini audio activity: "+err.Error(), false)
	}
}

func (c *client) handleAction(message incomingMessage) {
	c.stateMu.Lock()
	orch := c.orch
	c.stateMu.Unlock()
	if orch == nil {
		c.sendError("action received before a live session was started", false)
		return
	}
	orch.HandleAction(orchestrator.Action{
		SurfaceID:         message.SurfaceID,
		Name:              message.Name,
		SourceComponentID: message.SourceComponentID,
		Context:           message.Context,
	})
}

func (c *client) startLive(browserAPIKey string) {
	auth := authForStart(c.h.cfg.Auth, browserAPIKey)
	if !auth.AuthConfigured() {
		c.sendError(authconfig.MissingMessage(), true)
		return
	}
	clientConfig, err := auth.ClientConfig()
	if err != nil {
		c.sendError(err.Error(), true)
		return
	}
	c.stopLive()

	ctx, cancel := context.WithCancel(context.Background())
	liveModel, err := gemini.NewModel(ctx, c.h.cfg.LiveModel, clientConfig)
	if err != nil {
		cancel()
		c.sendError("create live model: "+err.Error(), true)
		return
	}
	cardModel, err := gemini.NewModel(ctx, c.h.cfg.CardModel, clientConfig)
	if err != nil {
		cancel()
		c.sendError("create card model: "+err.Error(), true)
		return
	}

	orch, err := orchestrator.New(orchestrator.Config{
		CardModel:      cardModel,
		Validator:      c.h.validator,
		Registry:       c.h.registry,
		Parent:         ctx,
		DeliverFinding: c.deliverFinding,
		Refreshes:      c.h.refreshes,
		Hooks: orchestrator.Hooks{
			Status: func(status orchestrator.TaskStatus) {
				c.h.broadcastTaskStatus(status)
			},
			Pending: func(pending orchestrator.CardPending) {
				frame := map[string]any{
					"type": "card_pending", "surfaceId": pending.SurfaceID,
					"taskId": pending.TaskID,
				}
				if pending.Title != "" {
					frame["title"] = pending.Title
				}
				c.h.broadcastJSON(frame)
			},
			A2UI: func(surfaceID string, messages []map[string]any) {
				if err := c.h.validator.ValidateMessages(messages); err != nil {
					c.sendError("blocked invalid outbound A2UI: "+err.Error(), false)
					return
				}
				c.h.broadcastJSON(map[string]any{"type": "a2ui", "surfaceId": surfaceID, "messages": messages})
			},
			CardRemoved: func(surfaceID string) {
				c.h.broadcastJSON(map[string]any{"type": "card_removed", "surfaceId": surfaceID})
			},
			Layout: func(slots []registry.Slot) {
				c.h.broadcastJSON(map[string]any{"type": "layout", "slots": slots})
			},
			Usage: func(inputTokens, outputTokens int, source string) {
				c.h.broadcastUsage(inputTokens, outputTokens, source)
			},
			Error: func(err error) {
				c.sendError(err.Error(), false)
			},
		},
	})
	if err != nil {
		cancel()
		c.sendError("create card orchestrator: "+err.Error(), true)
		return
	}

	dispatchTool, err := newDispatchTool(orch)
	if err != nil {
		orch.Close()
		cancel()
		c.sendError("create dispatch_task tool: "+err.Error(), true)
		return
	}
	listCardsTool, err := newListCardsTool(c.h.registry)
	if err != nil {
		orch.Close()
		cancel()
		c.sendError("create list_cards tool: "+err.Error(), true)
		return
	}
	voiceAgent, err := llmagent.New(llmagent.Config{
		Name:        "voice-front-door",
		Description: "Routes dashboard requests to asynchronous card tasks.",
		Model:       liveModel,
		Instruction: liveAgentInstruction,
		Tools:       []tool.Tool{dispatchTool, listCardsTool},
	})
	if err != nil {
		orch.Close()
		cancel()
		c.sendError("create voice agent: "+err.Error(), true)
		return
	}
	sessionService := session.InMemoryService()
	adkRunner, err := runner.New(runner.Config{
		AppName:        "voice2canvas-live",
		Agent:          voiceAgent,
		SessionService: sessionService,
	})
	if err != nil {
		orch.Close()
		cancel()
		c.sendError("create live runner: "+err.Error(), true)
		return
	}

	sessionID := uuid.NewString()
	if _, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName:   "voice2canvas-live",
		UserID:    "browser",
		SessionID: sessionID,
	}); err != nil {
		orch.Close()
		cancel()
		c.sendError("create live session: "+err.Error(), true)
		return
	}
	live, events, err := adkRunner.RunLive(ctx, "browser", sessionID, agent.LiveRunConfig{
		ResponseModalities: []genai.Modality{genai.ModalityAudio},
		InputAudioTranscription: &genai.AudioTranscriptionConfig{
			LanguageAuto: &genai.LanguageAuto{},
		},
		OutputAudioTranscription: &genai.AudioTranscriptionConfig{
			LanguageAuto: &genai.LanguageAuto{},
		},
		RealtimeInputConfig: &genai.RealtimeInputConfig{
			AutomaticActivityDetection: &genai.AutomaticActivityDetection{Disabled: false},
		},
	})
	if err != nil {
		orch.Close()
		cancel()
		c.sendError("start Gemini Live session: "+err.Error(), true)
		return
	}

	c.stateMu.Lock()
	c.live = live
	c.cancel = cancel
	c.orch = orch
	c.stateMu.Unlock()
	c.sendJSON(map[string]any{"type": "ready", "sessionId": sessionID})
	c.h.replayDashboard(c)
	go c.forwardLiveEvents(events)
}

func authForStart(fallback authconfig.Config, browserAPIKey string) authconfig.Config {
	if key := strings.TrimSpace(browserAPIKey); key != "" {
		return authconfig.Config{APIKey: key}
	}
	return fallback
}

const liveAgentInstruction = `You are the voice front-door of a test dashboard.
For every actionable user request you MUST call dispatch_task. Multiple requests in one utterance require multiple dispatch_task calls, one per request. Use intent create for new cards, update for card changes, remove for deletion, investigate for explain/why/compare/summarize questions about dashboard data or cards, and arrange for requests to arrange, organize, tidy, reorder, move one card relative to another, or make a card bigger. Arrange may target the whole canvas or particular cards. For an investigation, tell the user briefly that you are looking into it, call dispatch_task, and keep listening; do not answer it yourself because the investigator's result will arrive in a later text turn for you to relay conversationally.

Set domain to "weather" for conditions, forecasts, precipitation, temperature, wind, or weather alerts. Set domain to "markets" for stocks, indexes, funds, crypto, currencies, commodities, company results, or market comparisons. Use domain "general" for every other request.

Before every update, remove, arrange, or card-focused investigation, call list_cards to resolve references such as "that card", "the weather one", or "the second card". Pass the exact returned surfaceId as targetCard when one card is targeted; never guess or invent a surface ID. For arrange requests involving multiple cards, include all resolved surface IDs in the concrete description. When the user asks to "keep an eye on", "keep it updated", "auto-update", "monitor", or otherwise continuously refresh a card, set refreshSeconds between 30 and 3600. The dispatch_task tool only acknowledges immediately; card generation, refresh, investigation, and curation continue asynchronously. The list_cards result is an object with "count" and a "cards" array; count 0 means the dashboard is empty — say so plainly, never report a tool problem. Because generation is asynchronous, a just-dispatched card may take a few seconds to appear in list_cards; if the user asks to modify or remove a card you only just created, call list_cards again before concluding it does not exist.`

type dispatchTaskArgs struct {
	Intent         string `json:"intent"`
	Description    string `json:"description"`
	TargetCard     string `json:"targetCard,omitempty"`
	RefreshSeconds int    `json:"refreshSeconds,omitempty"`
	Domain         string `json:"domain,omitempty"`
}

type dispatchTaskResult struct {
	Status string `json:"status"`
	TaskID string `json:"taskId"`
}

func newDispatchTool(orch *orchestrator.Orchestrator) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "dispatch_task",
		Description: "Fast-acknowledges one dashboard create, update, remove, investigation, or arrangement task.",
		InputSchema: dispatchTaskInputSchema(),
	}, func(_ agent.Context, args dispatchTaskArgs) (dispatchTaskResult, error) {
		log.Printf("tool dispatch_task intent=%q domain=%q target=%q refresh=%d desc=%q",
			args.Intent, args.Domain, args.TargetCard, args.RefreshSeconds, args.Description)
		taskID := orch.Dispatch(orchestrator.Task{
			Intent:         args.Intent,
			Description:    args.Description,
			TargetCard:     args.TargetCard,
			RefreshSeconds: args.RefreshSeconds,
			Domain:         args.Domain,
		})
		return dispatchTaskResult{Status: "dispatched", TaskID: taskID}, nil
	})
}

func dispatchTaskInputSchema() *jsonschema.Schema {
	minimum := float64(30)
	maximum := float64(3600)
	return &jsonschema.Schema{
		Type:     "object",
		Required: []string{"intent", "description"},
		Properties: map[string]*jsonschema.Schema{
			"intent":         {Type: "string", Enum: []any{"create", "update", "remove", "investigate", "arrange"}},
			"description":    {Type: "string", Description: "Concrete dashboard task, investigation question, or arrangement instruction."},
			"targetCard":     {Type: "string", Description: "Exact surfaceId returned by list_cards."},
			"refreshSeconds": {Type: "integer", Minimum: &minimum, Maximum: &maximum, Description: "Optional continuous refresh interval in seconds."},
			"domain":         {Type: "string", Enum: []any{"general", "weather", "markets"}, Description: "Use weather for conditions and forecasts, markets for public financial-market data, and general otherwise."},
		},
	}
}

type listCardsArgs struct{}

// listCardsResponse wraps the inventory in an object: ADK's slice-return
// fallback path proved unreliable on the live connection (the model reported
// tool failure despite a successful handler), and object results convert
// cleanly without hitting that fallback.
type listCardsResponse struct {
	Count int              `json:"count"`
	Cards []listCardResult `json:"cards"`
}

type listCardResult struct {
	SurfaceID   string `json:"surfaceId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	CreatedAt   string `json:"createdAt"`
	Order       int    `json:"order"`
	Span        int    `json:"span"`
}

func newListCardsTool(cardRegistry *registry.Registry) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "list_cards",
		Description: "Returns the current dashboard card inventory in layout order so exact surface IDs can be resolved without guessing.",
	}, func(_ agent.Context, _ listCardsArgs) (listCardsResponse, error) {
		inventory := listCardInventory(cardRegistry)
		ids := make([]string, 0, len(inventory))
		for _, card := range inventory {
			ids = append(ids, card.SurfaceID)
		}
		log.Printf("tool list_cards → %d cards %v", len(inventory), ids)
		return listCardsResponse{Count: len(inventory), Cards: inventory}, nil
	})
}

func listCardInventory(cardRegistry *registry.Registry) []listCardResult {
	cards := cardRegistry.Cards()
	result := make([]listCardResult, 0, len(cards))
	for _, card := range cards {
		result = append(result, listCardResult{
			SurfaceID: card.SurfaceID, Title: card.Title, Description: card.Description,
			CreatedAt: card.CreatedAt.UTC().Format(time.RFC3339Nano), Order: card.Order, Span: card.Span,
		})
	}
	return result
}

func (c *client) forwardLiveEvents(events iter.Seq2[*session.Event, error]) {
	for event, err := range events {
		if err != nil {
			c.sendError("Gemini Live receive: "+err.Error(), false)
			return
		}
		if event == nil {
			continue
		}
		if event.Interrupted {
			// Nothing is ever sent while the model speaks, so an interruption
			// is the user barging in; held findings are dropped in favour of
			// their turn.
			c.speech.Interrupted()
			c.sendJSON(map[string]any{"type": "interrupted"})
		}
		if event.TurnComplete {
			c.speech.TurnComplete()
		}
		if transcription := event.InputTranscription; transcription != nil && transcription.Text != "" {
			c.sendJSON(map[string]any{
				"type":  "input_transcript",
				"text":  transcription.Text,
				"final": transcription.Finished || !event.Partial,
			})
		}
		if transcription := event.OutputTranscription; transcription != nil && transcription.Text != "" {
			c.sendJSON(map[string]any{
				"type":  "output_transcript",
				"text":  transcription.Text,
				"final": transcription.Finished || !event.Partial,
			})
		}
		if event.Content == nil {
			continue
		}
		for _, part := range event.Content.Parts {
			if part == nil || part.InlineData == nil || len(part.InlineData.Data) == 0 {
				continue
			}
			// Live audio is emitted as raw PCM by Gemini; forward only the
			// bytes so the browser can play them according to PROTOCOL.md.
			if strings.HasPrefix(part.InlineData.MIMEType, "audio/") || part.InlineData.MIMEType == "" {
				c.speech.SpeechStarted()
				c.sendBinary(part.InlineData.Data)
			}
		}
	}
}

func (c *client) stopLive() {
	c.speech.Reset()
	c.liveSendMu.Lock()
	defer c.liveSendMu.Unlock()
	c.stateMu.Lock()
	live := c.live
	cancel := c.cancel
	orch := c.orch
	c.live = nil
	c.cancel = nil
	c.orch = nil
	c.stateMu.Unlock()
	if orch != nil {
		orch.Close()
	}
	if live != nil {
		_ = live.Close()
	}
	if cancel != nil {
		cancel()
	}
}

var errLiveSessionUnavailable = errors.New("live session is unavailable")

func (c *client) sendLiveRequest(request agent.LiveRequest) error {
	c.liveSendMu.Lock()
	defer c.liveSendMu.Unlock()
	c.stateMu.Lock()
	live := c.live
	c.stateMu.Unlock()
	if live == nil {
		return errLiveSessionUnavailable
	}
	return live.Send(request)
}

// deliverFinding hands the result to the speech queue rather than the session
// directly, so it waits for any in-progress turn instead of interrupting it.
// Delivery is therefore asynchronous and send failures surface via the queue's
// error callback.
func (c *client) deliverFinding(text string) error {
	c.speech.Enqueue(text)
	return nil
}

func (c *client) speakFinding(text string) error {
	err := c.sendLiveRequest(agent.LiveRequest{
		Content: genai.NewContentFromText(text, genai.RoleUser),
	})
	if errors.Is(err, errLiveSessionUnavailable) {
		// Investigations can finish after stop/disconnect; dashboard state and
		// task status still complete, but there is no voice session to notify.
		return nil
	}
	return err
}

func (c *client) sendError(message string, fatal bool) {
	c.sendJSON(map[string]any{"type": "error", "message": message, "fatal": fatal})
}

func (c *client) sendJSON(value any) {
	if c.sendJSONHook != nil {
		c.sendJSONHook(value)
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *client) sendBinary(data []byte) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.WriteMessage(websocket.BinaryMessage, append([]byte(nil), data...))
}

func (h *handler) addClient(client *client) {
	h.clientsMu.Lock()
	h.clients[client] = struct{}{}
	h.clientsMu.Unlock()
}

func (h *handler) removeClient(client *client) {
	h.clientsMu.Lock()
	delete(h.clients, client)
	h.clientsMu.Unlock()
}

func (h *handler) broadcastJSON(frame any) {
	h.broadcastMu.Lock()
	defer h.broadcastMu.Unlock()
	h.clientsMu.RLock()
	clients := make([]*client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.clientsMu.RUnlock()
	for _, client := range clients {
		client.sendJSON(frame)
	}
}

func (h *handler) broadcastTaskStatus(status orchestrator.TaskStatus) {
	frame := map[string]any{
		"type": "task_status", "taskId": status.TaskID, "status": status.Status,
	}
	if status.SurfaceID != "" {
		frame["surfaceId"] = status.SurfaceID
	}
	if status.Detail != "" {
		frame["detail"] = status.Detail
	}
	h.broadcastJSON(frame)
}

func (h *handler) broadcastUsage(inputTokens, outputTokens int, source string) {
	h.broadcastJSON(map[string]any{
		"type":         "usage",
		"inputTokens":  inputTokens,
		"outputTokens": outputTokens,
		"source":       source,
		"at":           time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func (h *handler) replayDashboard(client *client) {
	// Serialize replay with mutations so the client observes a coherent
	// bootstrap followed by one layout. Concurrent mutations may be replayed
	// and then broadcast again; A2UI bootstraps are intentionally idempotent.
	h.broadcastMu.Lock()
	defer h.broadcastMu.Unlock()
	for _, card := range h.registry.Cards() {
		messages := orchestrator.BuildBootstrapMessages(card)
		if err := h.validator.ValidateMessages(messages); err != nil {
			client.sendError("blocked invalid replay A2UI: "+err.Error(), false)
			continue
		}
		client.sendJSON(map[string]any{
			"type": "a2ui", "surfaceId": card.SurfaceID, "messages": messages,
		})
	}
	client.sendJSON(map[string]any{"type": "layout", "slots": h.registry.Layout()})
}
