// Package orchestrator dispatches card-generation work independently of the
// voice Live session. Dispatch returns immediately; generation happens in a
// bounded worker set and reports progress through Hooks.
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/adk/v2/tool/geminitool"
	"google.golang.org/genai"

	"voice2canvas/backend/internal/a2ui"
	"voice2canvas/backend/internal/registry"
)

const (
	IntentCreate      = "create"
	IntentUpdate      = "update"
	IntentRemove      = "remove"
	IntentInvestigate = "investigate"
	IntentArrange     = "arrange"
	DomainGeneral     = "general"
	DomainWeather     = "weather"
	DomainMarkets     = "markets"

	statusDispatched  = "dispatched"
	statusResearching = "researching"
	statusGenerating  = "generating"
	statusRendered    = "rendered"
	statusFailed      = "failed"
)

// Task is the normalized work item produced by dispatch_task or a card action.
type Task struct {
	ID             string
	Intent         string
	Description    string
	TargetCard     string
	RefreshSeconds int
	Domain         string

	reserved *registry.Card
}

// Action is the application-level action received from an A2UI card.
type Action struct {
	SurfaceID         string
	Name              string
	SourceComponentID string
	Context           map[string]any
}

type TaskStatus struct {
	TaskID    string
	SurfaceID string
	Status    string
	Detail    string
}

// CardPending reserves a dashboard slot while a card is being generated.
type CardPending struct {
	SurfaceID string
	TaskID    string
	Title     string
}

// Hooks are called after work has passed the relevant validation boundary. A
// WebSocket connection supplies these callbacks to turn them into protocol
// frames.
type Hooks struct {
	Status      func(TaskStatus)
	Pending     func(CardPending)
	A2UI        func(surfaceID string, messages []map[string]any)
	CardRemoved func(surfaceID string)
	Layout      func([]registry.Slot)
	Usage       func(inputTokens, outputTokens int, source string)
	Error       func(error)
}

type Config struct {
	CardModel      model.LLM
	Validator      *a2ui.Validator
	Registry       *registry.Registry
	Hooks          Hooks
	Concurrency    int
	Parent         context.Context
	HTTPClient     *http.Client
	DeliverFinding func(string) error
	Refreshes      *RefreshCoordinator

	tickerFactory func(time.Duration) refreshTicker
}

type Orchestrator struct {
	runner         *runner.Runner
	researchRunner *runner.Runner
	weatherRunner  *runner.Runner
	marketRunner   *runner.Runner
	analystRunner  *runner.Runner
	curatorRunner  *runner.Runner
	sessions       session.Service
	validator      *a2ui.Validator
	registry       *registry.Registry
	hooks          Hooks
	semaphore      chan struct{}
	ctx            context.Context
	cancel         context.CancelFunc
	deliverFinding func(string) error
	tickerFactory  func(time.Duration) refreshTicker
	refreshes      *RefreshCoordinator
}

type refreshTicker interface {
	C() <-chan time.Time
	Stop()
}

type realRefreshTicker struct {
	ticker *time.Ticker
}

func (t *realRefreshTicker) C() <-chan time.Time { return t.ticker.C }
func (t *realRefreshTicker) Stop()               { t.ticker.Stop() }

type refreshLoop struct {
	cancel context.CancelFunc
	seq    uint64
	taskID string
	domain string
	owner  *Orchestrator
}

const maxRefreshingCards = 4

// RefreshCoordinator tracks the process-global set of refreshing surfaces.
// Individual loops still execute on, and are closed with, their owning
// Orchestrator.
type RefreshCoordinator struct {
	mu         sync.Mutex
	refreshers map[string]*refreshLoop
	nextSeq    uint64
}

func NewRefreshCoordinator() *RefreshCoordinator {
	return &RefreshCoordinator{refreshers: make(map[string]*refreshLoop)}
}

// New constructs the card, researcher, investigator, and curator agents for
// one live connection.
// Sessions are unique per task, allowing Dispatch to fan out concurrently.
func New(cfg Config) (*Orchestrator, error) {
	if cfg.CardModel == nil {
		return nil, fmt.Errorf("card model is required")
	}
	validator := cfg.Validator
	if validator == nil {
		var err error
		validator, err = a2ui.NewValidator()
		if err != nil {
			return nil, err
		}
	}
	cardRegistry := cfg.Registry
	if cardRegistry == nil {
		cardRegistry = registry.New()
	}
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	parent := cfg.Parent
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)

	httpTool, err := newHTTPGetTool(cfg.HTTPClient)
	if err != nil {
		cancel()
		return nil, err
	}
	cardAgent, err := llmagent.New(llmagent.Config{
		Name:        "card-generator",
		Description: "Generates one validated dashboard card envelope from a task.",
		Model:       cfg.CardModel,
		InstructionProvider: func(agent.ReadonlyContext) (string, error) {
			return cardAgentInstruction, nil
		},
		Tools: []tool.Tool{httpTool},
		Mode:  llmagent.ModeChat,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create card-generator agent: %w", err)
	}
	researchAgent, err := llmagent.New(llmagent.Config{
		Name:        "researcher",
		Description: "Searches for current facts and produces a compact dashboard-card data brief.",
		Model:       cfg.CardModel,
		InstructionProvider: func(agent.ReadonlyContext) (string, error) {
			return researcherAgentInstruction, nil
		},
		Tools: []tool.Tool{geminitool.GoogleSearch{}},
		Mode:  llmagent.ModeChat,
		// Gemini requires this opt-in whenever built-in tools (Google Search)
		// can appear alongside function declarations in one request.
		GenerateContentConfig: &genai.GenerateContentConfig{
			ToolConfig: &genai.ToolConfig{IncludeServerSideToolInvocations: genai.Ptr(true)},
		},
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create researcher agent: %w", err)
	}
	weatherAgent, err := llmagent.New(llmagent.Config{
		Name:        "weather-specialist",
		Description: "Fetches current conditions and forecasts from Open-Meteo and produces a compact data brief.",
		Model:       cfg.CardModel,
		InstructionProvider: func(agent.ReadonlyContext) (string, error) {
			return weatherAgentInstruction, nil
		},
		Tools: []tool.Tool{httpTool},
		Mode:  llmagent.ModeChat,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create weather specialist: %w", err)
	}
	marketAgent, err := llmagent.New(llmagent.Config{
		Name:        "market-specialist",
		Description: "Researches current public market data and produces a sourced, time-stamped data brief.",
		Model:       cfg.CardModel,
		InstructionProvider: func(agent.ReadonlyContext) (string, error) {
			return marketAgentInstruction, nil
		},
		Tools: []tool.Tool{geminitool.GoogleSearch{}},
		Mode:  llmagent.ModeChat,
		GenerateContentConfig: &genai.GenerateContentConfig{
			ToolConfig: &genai.ToolConfig{IncludeServerSideToolInvocations: genai.Ptr(true)},
		},
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create market specialist: %w", err)
	}
	analystAgent, err := llmagent.New(llmagent.Config{
		Name:        "investigator",
		Description: "Investigates dashboard data and returns a concise spoken finding.",
		Model:       cfg.CardModel,
		InstructionProvider: func(agent.ReadonlyContext) (string, error) {
			return investigatorAgentInstruction, nil
		},
		Tools: []tool.Tool{geminitool.GoogleSearch{}, httpTool},
		Mode:  llmagent.ModeChat,
		GenerateContentConfig: &genai.GenerateContentConfig{
			ToolConfig: &genai.ToolConfig{IncludeServerSideToolInvocations: genai.Ptr(true)},
		},
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create investigator agent: %w", err)
	}
	curatorAgent, err := llmagent.New(llmagent.Config{
		Name:        "layout-curator",
		Description: "Arranges every live dashboard card and returns a validated layout.",
		Model:       cfg.CardModel,
		InstructionProvider: func(agent.ReadonlyContext) (string, error) {
			return curatorAgentInstruction, nil
		},
		Mode: llmagent.ModeChat,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create layout-curator agent: %w", err)
	}
	sessionService := session.InMemoryService()
	adkRunner, err := runner.New(runner.Config{
		AppName:        "voice2canvas-card-generator",
		Agent:          cardAgent,
		SessionService: sessionService,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create card-generator runner: %w", err)
	}
	researchRunner, err := runner.New(runner.Config{
		AppName:        "voice2canvas-researcher",
		Agent:          researchAgent,
		SessionService: sessionService,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create researcher runner: %w", err)
	}
	weatherRunner, err := runner.New(runner.Config{
		AppName:        "voice2canvas-weather",
		Agent:          weatherAgent,
		SessionService: sessionService,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create weather runner: %w", err)
	}
	marketRunner, err := runner.New(runner.Config{
		AppName:        "voice2canvas-markets",
		Agent:          marketAgent,
		SessionService: sessionService,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create market runner: %w", err)
	}
	analystRunner, err := runner.New(runner.Config{
		AppName:        "voice2canvas-investigator",
		Agent:          analystAgent,
		SessionService: sessionService,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create investigator runner: %w", err)
	}
	curatorRunner, err := runner.New(runner.Config{
		AppName:        "voice2canvas-layout-curator",
		Agent:          curatorAgent,
		SessionService: sessionService,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create layout-curator runner: %w", err)
	}
	tickerFactory := cfg.tickerFactory
	if tickerFactory == nil {
		tickerFactory = func(interval time.Duration) refreshTicker {
			return &realRefreshTicker{ticker: time.NewTicker(interval)}
		}
	}
	refreshes := cfg.Refreshes
	if refreshes == nil {
		refreshes = NewRefreshCoordinator()
	}

	return &Orchestrator{
		runner:         adkRunner,
		researchRunner: researchRunner,
		weatherRunner:  weatherRunner,
		marketRunner:   marketRunner,
		analystRunner:  analystRunner,
		curatorRunner:  curatorRunner,
		sessions:       sessionService,
		validator:      validator,
		registry:       cardRegistry,
		hooks:          cfg.Hooks,
		semaphore:      make(chan struct{}, concurrency),
		ctx:            ctx,
		cancel:         cancel,
		deliverFinding: cfg.DeliverFinding,
		tickerFactory:  tickerFactory,
		refreshes:      refreshes,
	}, nil
}

func (o *Orchestrator) Registry() *registry.Registry {
	return o.registry
}

// Close cancels queued and in-flight work and every auto-refresh loop.
func (o *Orchestrator) Close() {
	if o == nil {
		return
	}
	o.stopAllRefreshers()
	if o.cancel != nil {
		o.cancel()
	}
}

// Dispatch emits the fast acknowledgement status and starts work without
// waiting for the model or HTTP tool.
func (o *Orchestrator) Dispatch(task Task) string {
	if task.ID == "" {
		task.ID = uuid.NewString()
	}
	o.emitStatus(TaskStatus{TaskID: task.ID, Status: statusDispatched})
	if task.Intent == IntentCreate {
		reserved := o.registry.Reserve()
		task.reserved = &reserved
		o.emitPending(CardPending{SurfaceID: reserved.SurfaceID, TaskID: task.ID, Title: pendingTitle(task.Description)})
	}
	go o.runTask(task)
	return task.ID
}

// HandleAction translates a card action into an update task. The test build
// treats all actions as update requests and includes action context verbatim.
func (o *Orchestrator) HandleAction(action Action) string {
	contextJSON, err := json.Marshal(action.Context)
	if err != nil {
		contextJSON = []byte("{}")
	}
	description := fmt.Sprintf("Card action %q from component %q. Action context: %s", action.Name, action.SourceComponentID, contextJSON)
	return o.Dispatch(Task{
		Intent:      IntentUpdate,
		Description: description,
		TargetCard:  action.SurfaceID,
	})
}

func (o *Orchestrator) runTask(task Task) {
	select {
	case o.semaphore <- struct{}{}:
		defer func() { <-o.semaphore }()
	case <-o.ctx.Done():
		o.emitStatus(TaskStatus{TaskID: task.ID, Status: statusFailed, Detail: "orchestrator closed"})
		return
	}

	switch task.Intent {
	case IntentRemove:
		o.emitStatus(TaskStatus{TaskID: task.ID, Status: statusGenerating})
		o.remove(task)
	case IntentCreate, IntentUpdate:
		dataBrief := ""
		if task.Domain == DomainWeather || task.Domain == DomainMarkets || shouldResearch(task) {
			surfaceID := task.TargetCard
			if task.reserved != nil {
				surfaceID = task.reserved.SurfaceID
			}
			o.emitStatus(TaskStatus{TaskID: task.ID, SurfaceID: surfaceID, Status: statusResearching})
			var existing *CardEnvelope
			if card, ok := o.registry.Resolve(task.TargetCard); ok {
				existing = envelopeFromCard(card)
			}
			brief, stage, err := o.runDataAgent(task, existing)
			if err != nil {
				detail := stage + " unavailable; continuing with card generation: " + err.Error()
				slog.Warn("data agent unavailable", "stage", stage, "task", task.ID, "error", err)
				o.emitStatus(TaskStatus{TaskID: task.ID, SurfaceID: surfaceID, Status: statusResearching, Detail: detail})
			} else {
				dataBrief = strings.TrimSpace(brief)
			}
		}
		o.emitStatus(TaskStatus{TaskID: task.ID, Status: statusGenerating})
		o.createOrUpdate(task, dataBrief)
	case IntentInvestigate:
		dataBrief := ""
		if task.Domain == DomainWeather || task.Domain == DomainMarkets {
			o.emitStatus(TaskStatus{TaskID: task.ID, SurfaceID: task.TargetCard, Status: statusResearching})
			brief, stage, err := o.runDataAgent(task, nil)
			if err != nil {
				detail := stage + " unavailable; continuing with investigation: " + err.Error()
				slog.Warn("data agent unavailable", "stage", stage, "task", task.ID, "error", err)
				o.emitStatus(TaskStatus{TaskID: task.ID, SurfaceID: task.TargetCard, Status: statusResearching, Detail: detail})
				dataBrief = detail
			} else {
				dataBrief = strings.TrimSpace(brief)
			}
		}
		o.emitStatus(TaskStatus{TaskID: task.ID, Status: statusGenerating})
		o.investigate(task, dataBrief)
	case IntentArrange:
		o.emitStatus(TaskStatus{TaskID: task.ID, Status: statusGenerating})
		o.arrange(task)
	default:
		o.emitStatus(TaskStatus{TaskID: task.ID, Status: statusFailed, Detail: "intent must be create, update, remove, investigate, or arrange"})
	}
}

func (o *Orchestrator) remove(task Task) {
	card, ok := o.registry.Resolve(task.TargetCard)
	if !ok {
		o.emitStatus(TaskStatus{TaskID: task.ID, Status: statusFailed, Detail: "target card not found"})
		return
	}
	messages := []map[string]any{{
		"version":       a2ui.Version,
		"deleteSurface": map[string]any{"surfaceId": card.SurfaceID},
	}}
	if err := o.validator.ValidateMessages(messages); err != nil {
		o.emitStatus(TaskStatus{TaskID: task.ID, SurfaceID: card.SurfaceID, Status: statusFailed, Detail: err.Error()})
		return
	}
	o.stopRefresher(card.SurfaceID)
	o.emitA2UI(card.SurfaceID, messages)
	o.registry.Remove(card.SurfaceID)
	if o.hooks.CardRemoved != nil {
		o.hooks.CardRemoved(card.SurfaceID)
	}
	o.emitLayout()
	o.emitStatus(TaskStatus{TaskID: task.ID, SurfaceID: card.SurfaceID, Status: statusRendered})
}

func (o *Orchestrator) createOrUpdate(task Task, dataBrief string) {
	var (
		card     registry.Card
		existing *CardEnvelope
		isCreate = task.Intent == IntentCreate
	)
	if isCreate {
		if task.reserved == nil {
			card = o.registry.Reserve()
			o.emitPending(CardPending{SurfaceID: card.SurfaceID, TaskID: task.ID, Title: pendingTitle(task.Description)})
		} else {
			card = *task.reserved
		}
	} else {
		var ok bool
		card, ok = o.registry.Resolve(task.TargetCard)
		if !ok {
			o.emitStatus(TaskStatus{TaskID: task.ID, Status: statusFailed, Detail: "target card not found"})
			return
		}
		existing = envelopeFromCard(card)
	}

	generated, rawOutput, generationErr := o.generateEnvelope(task, existing, dataBrief)
	if generationErr == nil {
		effective, stateErr := materializeEnvelope(generated, existing, isCreate)
		if stateErr != nil {
			generationErr = stateErr
		} else {
			wireEnvelope := generated
			if isCreate {
				wireEnvelope = effective
			}
			messages := buildCardMessages(card.SurfaceID, isCreate, wireEnvelope)
			if validationErr := o.validator.ValidateMessages(messages); validationErr == nil {
				o.commitAndRender(task, card, effective, messages, isCreate, true)
				return
			} else {
				generationErr = validationErr
			}
		}
	}

	// Malformed model text and invalid A2UI output both get one repair
	// round-trip. A transport error is also eligible for the same retry; if it
	// fails again, the hardcoded fallback below remains the visible result.
	if generationErr != nil {
		repaired, repairErr := o.repairEnvelope(rawOutput, generationErr, dataBrief)
		if repairErr == nil {
			effective, stateErr := materializeEnvelope(repaired, existing, isCreate)
			if stateErr != nil {
				generationErr = stateErr
			} else {
				wireEnvelope := repaired
				if isCreate {
					wireEnvelope = effective
				}
				messages := buildCardMessages(card.SurfaceID, isCreate, wireEnvelope)
				if validationErr := o.validator.ValidateMessages(messages); validationErr == nil {
					o.commitAndRender(task, card, effective, messages, isCreate, true)
					return
				} else {
					generationErr = validationErr
				}
			}
		} else {
			generationErr = repairErr
		}
	}

	if generationErr == nil {
		// The branches above return after a successful commit. This guard keeps
		// the fallback detail meaningful if that control flow is changed later.
		generationErr = fmt.Errorf("card generation failed")
	}

	// A model/network/repair failure still produces a validated, visible error
	// card so the dashboard does not silently lose the user's request.
	detail := strings.TrimSpace(generationErr.Error())
	if detail == "" {
		detail = "card generation failed"
	}
	fallback := fallbackEnvelope(detail)
	messages := buildCardMessages(card.SurfaceID, isCreate, fallback)
	if validationErr := o.validator.ValidateMessages(messages); validationErr != nil {
		o.emitStatus(TaskStatus{TaskID: task.ID, SurfaceID: card.SurfaceID, Status: statusFailed, Detail: validationErr.Error()})
		o.emitError(validationErr)
		return
	}
	o.emitStatus(TaskStatus{TaskID: task.ID, SurfaceID: card.SurfaceID, Status: statusFailed, Detail: detail})
	o.commitAndRender(task, card, fallback, messages, isCreate, false)
}

func envelopeFromCard(card registry.Card) *CardEnvelope {
	return &CardEnvelope{
		Title:       card.Title,
		Description: card.Description,
		Components:  card.Components,
		DataModel:   card.DataModelSummary,
	}
}

func materializeEnvelope(generated CardEnvelope, existing *CardEnvelope, create bool) (CardEnvelope, error) {
	if create {
		if generated.Title == "" {
			return CardEnvelope{}, fmt.Errorf("card envelope title is required for create")
		}
		if len(generated.Components) == 0 {
			return CardEnvelope{}, fmt.Errorf("card envelope components are required for create")
		}
		effective := cloneEnvelope(generated)
		if effective.DataModel == nil {
			effective.DataModel = map[string]any{}
		}
		patched, err := applyDataModelPatches(effective.DataModel, effective.DataModelPatches)
		if err != nil {
			return CardEnvelope{}, err
		}
		effective.DataModel = patched
		effective.DataModelPatches = nil
		heading, err := titleFromComponents(effective.Components)
		if err != nil {
			return CardEnvelope{}, err
		}
		effective.Title = heading
		return effective, nil
	}

	if existing == nil {
		return CardEnvelope{}, fmt.Errorf("existing card state is required for update")
	}
	if generated.Components == nil && generated.DataModel == nil && len(generated.DataModelPatches) == 0 {
		return CardEnvelope{}, fmt.Errorf("update envelope must change components or data model")
	}
	if generated.Components != nil && len(generated.Components) == 0 {
		return CardEnvelope{}, fmt.Errorf("update envelope components cannot be empty")
	}

	effective := cloneEnvelope(*existing)
	if generated.Title != "" {
		effective.Title = generated.Title
	}
	if generated.Description != "" {
		effective.Description = generated.Description
	}
	if generated.Components != nil {
		effective.Components = cloneEnvelope(generated).Components
	}
	if generated.DataModel != nil {
		effective.DataModel = cloneEnvelope(generated).DataModel
	}
	if effective.DataModel == nil {
		effective.DataModel = map[string]any{}
	}
	patched, err := applyDataModelPatches(effective.DataModel, generated.DataModelPatches)
	if err != nil {
		return CardEnvelope{}, err
	}
	effective.DataModel = patched
	effective.DataModelPatches = nil
	heading, err := titleFromComponents(effective.Components)
	if err != nil {
		return CardEnvelope{}, err
	}
	effective.Title = heading
	return effective, nil
}

// titleFromComponents returns the literal heading rendered at the top of a
// card. In the usual Card -> Column shape, "first visual child" means the
// Column's first child; root Columns are supported as well.
func titleFromComponents(components []map[string]any) (string, error) {
	byID := make(map[string]map[string]any, len(components))
	for _, component := range components {
		id, _ := component["id"].(string)
		if id != "" {
			byID[id] = component
		}
	}
	root := byID["root"]
	if root == nil {
		return "", fmt.Errorf("card components must contain root")
	}

	first, err := firstVisualChild(root, byID)
	if err != nil {
		return "", err
	}
	if component, _ := first["component"].(string); component != "Text" {
		return "", fmt.Errorf("first visual child inside root must be a heading Text component")
	}
	variant, _ := first["variant"].(string)
	if variant != "h1" && variant != "h2" && variant != "h3" && variant != "h4" && variant != "h5" {
		return "", fmt.Errorf("first visual child inside root must use heading Text variant h1 through h5")
	}
	title, ok := first["text"].(string)
	title = strings.TrimSpace(title)
	if !ok || title == "" {
		return "", fmt.Errorf("card heading Text must contain literal text")
	}
	if len(strings.Fields(title)) > 5 {
		return "", fmt.Errorf("card heading Text must contain 5 words or fewer")
	}
	return title, nil
}

func firstVisualChild(root map[string]any, byID map[string]map[string]any) (map[string]any, error) {
	container := root
	if childID, ok := root["child"].(string); ok && childID != "" {
		child := byID[childID]
		if child == nil {
			return nil, fmt.Errorf("root child %q is missing", childID)
		}
		if component, _ := child["component"].(string); component == "Text" {
			return child, nil
		}
		container = child
	}
	children, ok := container["children"].([]any)
	if !ok || len(children) == 0 {
		return nil, fmt.Errorf("card root content must begin with a heading Text child")
	}
	firstID, ok := children[0].(string)
	if !ok || firstID == "" {
		return nil, fmt.Errorf("card root content first child must reference a heading Text component")
	}
	first := byID[firstID]
	if first == nil {
		return nil, fmt.Errorf("card root content first child %q is missing", firstID)
	}
	return first, nil
}

func cloneEnvelope(envelope CardEnvelope) CardEnvelope {
	data, err := json.Marshal(envelope)
	if err != nil {
		return CardEnvelope{}
	}
	var clone CardEnvelope
	if err := json.Unmarshal(data, &clone); err != nil {
		return CardEnvelope{}
	}
	return clone
}

func applyDataModelPatches(model map[string]any, patches []DataModelPatch) (map[string]any, error) {
	cloned := cloneEnvelope(CardEnvelope{DataModel: model}).DataModel
	if cloned == nil {
		cloned = map[string]any{}
	}
	for index, patch := range patches {
		tokens, err := jsonPointerTokens(patch.Path)
		if err != nil {
			return nil, fmt.Errorf("dataModelPatches[%d]: %w", index, err)
		}
		var current any = cloned
		for tokenIndex, token := range tokens {
			last := tokenIndex == len(tokens)-1
			switch node := current.(type) {
			case map[string]any:
				if last {
					node[token] = cloneJSONValue(patch.Value)
					continue
				}
				next, ok := node[token]
				if !ok || next == nil {
					next = map[string]any{}
					node[token] = next
				}
				current = next
			case []any:
				arrayIndex, parseErr := strconv.Atoi(token)
				if parseErr != nil || arrayIndex < 0 || arrayIndex >= len(node) {
					return nil, fmt.Errorf("path %q has invalid array index %q", patch.Path, token)
				}
				if last {
					node[arrayIndex] = cloneJSONValue(patch.Value)
					continue
				}
				current = node[arrayIndex]
			default:
				return nil, fmt.Errorf("path %q traverses a scalar at %q", patch.Path, token)
			}
		}
	}
	return cloned, nil
}

func jsonPointerTokens(path string) ([]string, error) {
	if !strings.HasPrefix(path, "/") || path == "/" {
		return nil, fmt.Errorf("path %q must be a non-root JSON Pointer", path)
	}
	raw := strings.Split(path[1:], "/")
	tokens := make([]string, len(raw))
	for index, token := range raw {
		var decoded strings.Builder
		for offset := 0; offset < len(token); offset++ {
			if token[offset] != '~' {
				decoded.WriteByte(token[offset])
				continue
			}
			if offset+1 >= len(token) || (token[offset+1] != '0' && token[offset+1] != '1') {
				return nil, fmt.Errorf("path %q contains an invalid JSON Pointer escape", path)
			}
			offset++
			if token[offset] == '0' {
				decoded.WriteByte('~')
			} else {
				decoded.WriteByte('/')
			}
		}
		tokens[index] = decoded.String()
	}
	return tokens, nil
}

func cloneJSONValue(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var clone any
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil
	}
	return clone
}

func shouldResearch(task Task) bool {
	if task.Intent != IntentCreate && task.Intent != IntentUpdate {
		return false
	}
	description := strings.ToLower(strings.TrimSpace(task.Description))
	weatherTerms := []string{"weather", "forecast", "temperature", "humidity", "precipitation", "rain", "snow", "wind", "uv index"}
	nonWeatherDomains := []string{"stock", "share price", "revenue", "sales", "sports", "score", "news", "population", "traffic", "election", "air quality", "currency", "exchange rate", "crypto"}
	if containsAny(description, weatherTerms) && !containsAny(description, nonWeatherDomains) {
		return false
	}
	if task.Intent == IntentUpdate {
		cosmeticTerms := []string{"color", "colour", "style", "theme", "rename", "title", "label", "legend", "axis", "font", "chart type", "line chart", "bar chart", "table", "comparison", "make it wider", "make it smaller"}
		freshDataTerms := []string{"current", "latest", "new data", "refresh", "update data", "today", "live", "new value", "new figures", "as of"}
		if containsAny(description, cosmeticTerms) && !containsAny(description, freshDataTerms) {
			return false
		}
	}
	return true
}

func containsAny(value string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func (o *Orchestrator) generateEnvelope(task Task, existing *CardEnvelope, dataBrief string) (CardEnvelope, string, error) {
	raw, err := o.runCardAgent(cardPrompt(task, existing, dataBrief))
	if err != nil {
		return CardEnvelope{}, raw, err
	}
	envelope, err := parseEnvelope(raw)
	if err != nil {
		return CardEnvelope{}, raw, err
	}
	return envelope, raw, nil
}

func (o *Orchestrator) runResearcherAgent(prompt string) (string, error) {
	return o.runTextAgent(o.researchRunner, "voice2canvas-researcher", "researcher", "researcher", prompt, "researcher returned no text")
}

func (o *Orchestrator) runDataAgent(task Task, existing *CardEnvelope) (string, string, error) {
	switch task.Domain {
	case DomainWeather:
		brief, err := o.runTextAgent(o.weatherRunner, "voice2canvas-weather", "weather", "weather specialist", specialistPrompt("Weather", task, existing), "weather specialist returned no text")
		return brief, "weather data", err
	case DomainMarkets:
		brief, err := o.runTextAgent(o.marketRunner, "voice2canvas-markets", "markets", "market specialist", specialistPrompt("Market", task, existing), "market specialist returned no text")
		return brief, "market data", err
	default:
		brief, err := o.runResearcherAgent(researchPrompt(task, existing))
		return brief, "research", err
	}
}

func (o *Orchestrator) repairEnvelope(raw string, validationErr error, dataBrief string) (CardEnvelope, error) {
	repairPrompt := BuildRepairPrompt(raw, validationErr)
	if brief := strings.TrimSpace(dataBrief); brief != "" {
		repairPrompt += "\n\nRESEARCH DATA BRIEF (still authoritative; preserve its facts and cite its source names):\n" + brief
	}
	repairRaw, err := o.runCardAgent(repairPrompt)
	if err != nil {
		return CardEnvelope{}, fmt.Errorf("card repair call: %w", err)
	}
	envelope, err := parseEnvelope(repairRaw)
	if err != nil {
		return CardEnvelope{}, fmt.Errorf("parse repaired card envelope: %w", err)
	}
	return envelope, nil
}

func (o *Orchestrator) runCardAgent(prompt string) (string, error) {
	sessionID := uuid.NewString()
	if _, err := o.sessions.Create(o.ctx, &session.CreateRequest{
		AppName:   "voice2canvas-card-generator",
		UserID:    "card-generator",
		SessionID: sessionID,
	}); err != nil {
		return "", fmt.Errorf("create card session: %w", err)
	}
	usage := tokenUsage{}
	defer func() { o.emitUsage(usage) }()
	var output strings.Builder
	for event, err := range o.runner.Run(
		o.ctx,
		"card-generator",
		sessionID,
		genai.NewContentFromText(prompt, genai.RoleUser),
		agent.RunConfig{},
	) {
		if err != nil {
			return output.String(), err
		}
		usage.add(event)
		if event == nil || event.Content == nil {
			continue
		}
		for _, part := range event.Content.Parts {
			if part != nil && part.Text != "" {
				output.WriteString(part.Text)
			}
		}
	}
	if strings.TrimSpace(output.String()) == "" {
		return output.String(), fmt.Errorf("card agent returned no text")
	}
	return output.String(), nil
}

func (o *Orchestrator) investigate(task Task, dataBrief string) {
	cards := o.registry.Cards()
	if task.TargetCard != "" {
		if _, ok := o.registry.Resolve(task.TargetCard); !ok {
			o.emitStatus(TaskStatus{TaskID: task.ID, Status: statusFailed, Detail: "target card not found"})
			return
		}
	}
	finding, err := o.runInvestigatorAgent(investigationPrompt(task, cards, dataBrief))
	if err != nil {
		o.emitStatus(TaskStatus{TaskID: task.ID, SurfaceID: task.TargetCard, Status: statusFailed, Detail: err.Error()})
		return
	}
	finding = strings.TrimSpace(finding)
	if finding == "" {
		o.emitStatus(TaskStatus{TaskID: task.ID, SurfaceID: task.TargetCard, Status: statusFailed, Detail: "investigator returned no finding"})
		return
	}
	if o.deliverFinding != nil {
		content := "[investigation result — relay to the user conversationally, briefly] " + finding
		if err := o.deliverFinding(content); err != nil {
			o.emitError(fmt.Errorf("deliver investigation finding: %w", err))
		}
	}
	o.emitStatus(TaskStatus{TaskID: task.ID, SurfaceID: task.TargetCard, Status: statusRendered, Detail: finding})
}

func (o *Orchestrator) arrange(task Task) {
	cards := o.registry.Cards()
	if len(cards) == 0 {
		o.emitLayout()
		o.emitStatus(TaskStatus{TaskID: task.ID, Status: statusFailed, Detail: "there are no live cards to arrange"})
		return
	}
	raw, layoutErr := o.runCuratorAgent(curatorPrompt(task, cards))
	var layout curatedLayout
	if layoutErr == nil {
		layout, layoutErr = parseCuratedLayout(raw, cards)
	}
	if layoutErr != nil {
		repairRaw, repairErr := o.runCuratorAgent(buildCuratorRepairPrompt(raw, layoutErr, cards))
		if repairErr == nil {
			layout, repairErr = parseCuratedLayout(repairRaw, cards)
		}
		if repairErr != nil {
			layoutErr = fmt.Errorf("curator repair failed: %w", repairErr)
		} else {
			layoutErr = nil
		}
	}
	if layoutErr == nil {
		layoutErr = o.registry.ApplyLayout(layout.Slots)
	}
	if layoutErr != nil {
		o.emitLayout()
		o.emitStatus(TaskStatus{TaskID: task.ID, Status: statusFailed, Detail: layoutErr.Error()})
		return
	}

	o.emitLayout()
	if o.deliverFinding != nil {
		content := "[layout result — relay to the user conversationally, briefly] " + layout.Note
		if err := o.deliverFinding(content); err != nil {
			o.emitError(fmt.Errorf("deliver layout note: %w", err))
		}
	}
	o.emitStatus(TaskStatus{TaskID: task.ID, Status: statusRendered, Detail: layout.Note})
}

func (o *Orchestrator) runInvestigatorAgent(prompt string) (string, error) {
	return o.runTextAgent(o.analystRunner, "voice2canvas-investigator", "investigator", "investigator", prompt, "investigator returned no text")
}

func (o *Orchestrator) runCuratorAgent(prompt string) (string, error) {
	return o.runTextAgent(o.curatorRunner, "voice2canvas-layout-curator", "layout-curator", "layout-curator", prompt, "layout curator returned no text")
}

func (o *Orchestrator) runTextAgent(agentRunner *runner.Runner, appName, userID, agentName, prompt, emptyError string) (string, error) {
	sessionID := uuid.NewString()
	if _, err := o.sessions.Create(o.ctx, &session.CreateRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: sessionID,
	}); err != nil {
		return "", fmt.Errorf("create %s session: %w", agentName, err)
	}
	usage := tokenUsage{}
	defer func() { o.emitUsage(usage) }()
	var output strings.Builder
	for event, err := range agentRunner.Run(
		o.ctx,
		userID,
		sessionID,
		genai.NewContentFromText(prompt, genai.RoleUser),
		agent.RunConfig{},
	) {
		if err != nil {
			return output.String(), err
		}
		usage.add(event)
		if event == nil || event.Content == nil {
			continue
		}
		for _, part := range event.Content.Parts {
			if part != nil && part.Text != "" {
				output.WriteString(part.Text)
			}
		}
	}
	if strings.TrimSpace(output.String()) == "" {
		return output.String(), fmt.Errorf("%s", emptyError)
	}
	return output.String(), nil
}

type tokenUsage struct {
	inputTokens  int
	outputTokens int
	seen         bool
}

func (u *tokenUsage) add(event *session.Event) {
	if event == nil || event.UsageMetadata == nil {
		return
	}
	u.seen = true
	u.inputTokens += int(event.UsageMetadata.PromptTokenCount)
	// ADK's own telemetry follows OpenTelemetry and includes generated
	// reasoning tokens in output usage.
	u.outputTokens += int(event.UsageMetadata.CandidatesTokenCount + event.UsageMetadata.ThoughtsTokenCount)
}

func (o *Orchestrator) emitUsage(usage tokenUsage) {
	if usage.seen && o.hooks.Usage != nil {
		o.hooks.Usage(usage.inputTokens, usage.outputTokens, "worker")
	}
}

func (o *Orchestrator) startRefresher(surfaceID, taskID, domain string, seconds int) {
	if seconds < 30 || seconds > 3600 {
		o.emitStatus(TaskStatus{TaskID: taskID, SurfaceID: surfaceID, Status: statusFailed, Detail: "refreshSeconds must be between 30 and 3600"})
		return
	}
	ctx, cancel := context.WithCancel(o.ctx)
	ticker := o.tickerFactory(time.Duration(seconds) * time.Second)

	var evictedSurface string
	var evictedTask string
	o.refreshes.mu.Lock()
	if previous := o.refreshes.refreshers[surfaceID]; previous != nil {
		previous.cancel()
		delete(o.refreshes.refreshers, surfaceID)
	}
	if len(o.refreshes.refreshers) >= maxRefreshingCards {
		var oldestSurface string
		var oldest *refreshLoop
		for candidateSurface, candidate := range o.refreshes.refreshers {
			if oldest == nil || candidate.seq < oldest.seq {
				oldestSurface, oldest = candidateSurface, candidate
			}
		}
		if oldest != nil {
			oldest.cancel()
			delete(o.refreshes.refreshers, oldestSurface)
			evictedSurface, evictedTask = oldestSurface, oldest.taskID
		}
	}
	o.refreshes.nextSeq++
	loop := &refreshLoop{cancel: cancel, seq: o.refreshes.nextSeq, taskID: taskID, domain: domain, owner: o}
	o.refreshes.refreshers[surfaceID] = loop
	o.refreshes.mu.Unlock()

	if evictedSurface != "" {
		o.emitStatus(TaskStatus{
			TaskID: evictedTask, SurfaceID: evictedSurface, Status: statusRendered,
			Detail: "auto-refresh stopped because the four-card refresh limit was exceeded",
		})
	}
	go o.runRefresher(ctx, ticker, surfaceID, loop)
}

func (o *Orchestrator) runRefresher(ctx context.Context, ticker refreshTicker, surfaceID string, loop *refreshLoop) {
	defer ticker.Stop()
	defer func() {
		o.refreshes.mu.Lock()
		if current := o.refreshes.refreshers[surfaceID]; current == loop {
			delete(o.refreshes.refreshers, surfaceID)
		}
		o.refreshes.mu.Unlock()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			o.Dispatch(Task{
				Intent: IntentUpdate, TargetCard: surfaceID,
				Description: "refresh the card's data with current values; emit only updateDataModel patches",
				Domain:      loop.domain,
			})
		}
	}
}

func (o *Orchestrator) stopRefresher(surfaceID string) {
	o.refreshes.mu.Lock()
	loop := o.refreshes.refreshers[surfaceID]
	if loop != nil {
		delete(o.refreshes.refreshers, surfaceID)
		loop.cancel()
	}
	o.refreshes.mu.Unlock()
}

func (o *Orchestrator) stopAllRefreshers() {
	o.refreshes.mu.Lock()
	for surfaceID, loop := range o.refreshes.refreshers {
		if loop.owner == o {
			delete(o.refreshes.refreshers, surfaceID)
			loop.cancel()
		}
	}
	o.refreshes.mu.Unlock()
}

func pendingTitle(description string) string {
	title := strings.TrimSpace(description)
	for _, prefix := range []string{"please ", "create ", "make ", "add ", "show me ", "show "} {
		if strings.HasPrefix(strings.ToLower(title), prefix) {
			title = strings.TrimSpace(title[len(prefix):])
			break
		}
	}
	if end := strings.IndexAny(title, ".!?\n"); end >= 0 {
		title = strings.TrimSpace(title[:end])
	}
	const maxRunes = 48
	runes := []rune(title)
	if len(runes) > maxRunes {
		title = strings.TrimSpace(string(runes[:maxRunes-1])) + "…"
	}
	if title == "" {
		return "New card"
	}
	return title
}

func (o *Orchestrator) commitAndRender(task Task, reserved registry.Card, envelope CardEnvelope, messages []map[string]any, isCreate, reportRendered bool) {
	var card registry.Card
	if isCreate {
		card = o.registry.CommitState(reserved, envelope.Title, envelope.Description, envelope.Components, envelope.DataModel)
	} else {
		var ok bool
		card, ok = o.registry.UpsertState(reserved.SurfaceID, envelope.Title, envelope.Description, envelope.Components, envelope.DataModel)
		if !ok {
			o.emitStatus(TaskStatus{TaskID: task.ID, SurfaceID: reserved.SurfaceID, Status: statusFailed, Detail: "target card disappeared during generation"})
			return
		}
	}
	if !isCreate && containsComponentReplacement(messages) {
		o.emitPending(CardPending{SurfaceID: card.SurfaceID, TaskID: task.ID, Title: card.Title})
	}
	o.emitA2UI(card.SurfaceID, messages)
	o.emitLayout()
	if reportRendered {
		o.emitStatus(TaskStatus{TaskID: task.ID, SurfaceID: card.SurfaceID, Status: statusRendered})
		if task.RefreshSeconds > 0 {
			o.startRefresher(card.SurfaceID, task.ID, task.Domain, task.RefreshSeconds)
		}
	}
}

func containsComponentReplacement(messages []map[string]any) bool {
	for _, message := range messages {
		if _, ok := message["updateComponents"]; ok {
			return true
		}
	}
	return false
}

func buildCardMessages(surfaceID string, create bool, envelope CardEnvelope) []map[string]any {
	messages := make([]map[string]any, 0, 2+len(envelope.DataModelPatches))
	if create {
		messages = append(messages, map[string]any{
			"version": a2ui.Version,
			"createSurface": map[string]any{
				"surfaceId":     surfaceID,
				"catalogId":     a2ui.ExtendedCatalogID,
				"sendDataModel": true,
			},
		})
	}
	if create || envelope.Components != nil {
		components := make([]any, len(envelope.Components))
		for index := range envelope.Components {
			components[index] = envelope.Components[index]
		}
		messages = append(messages, map[string]any{
			"version": a2ui.Version,
			"updateComponents": map[string]any{
				"surfaceId":  surfaceID,
				"components": components,
			},
		})
	}
	if create || envelope.DataModel != nil {
		messages = append(messages, map[string]any{
			"version": a2ui.Version,
			"updateDataModel": map[string]any{
				"surfaceId": surfaceID,
				"value":     envelope.DataModel,
			},
		})
	}
	for _, patch := range envelope.DataModelPatches {
		messages = append(messages, map[string]any{
			"version": a2ui.Version,
			"updateDataModel": map[string]any{
				"surfaceId": surfaceID,
				"path":      patch.Path,
				"value":     patch.Value,
			},
		})
	}
	return messages
}

// BuildBootstrapMessages reconstructs an idempotent full A2UI bootstrap from
// the registry's effective rendered state for reconnect replay.
func BuildBootstrapMessages(card registry.Card) []map[string]any {
	return buildCardMessages(card.SurfaceID, true, CardEnvelope{
		Title: card.Title, Description: card.Description,
		Components: card.Components, DataModel: card.DataModelSummary,
	})
}

func fallbackEnvelope(detail string) CardEnvelope {
	return CardEnvelope{
		Title:       "Unable To Render Card",
		Description: "The card generator returned an error.",
		DataModel:   map[string]any{"error": detail, "sample": true},
		Components: []map[string]any{
			{"id": "root", "component": "Card", "child": "content"},
			{"id": "content", "component": "Column", "children": []any{"title", "description", "detail"}},
			{"id": "title", "component": "Text", "text": "Unable To Render Card", "variant": "h3"},
			{"id": "description", "component": "Text", "text": "The card generator returned an error.", "variant": "caption"},
			{"id": "detail", "component": "Text", "text": map[string]any{"path": "/error"}, "variant": "body"},
		},
	}
}

func (o *Orchestrator) emitStatus(status TaskStatus) {
	if o.hooks.Status != nil {
		o.hooks.Status(status)
	}
}

func (o *Orchestrator) emitPending(pending CardPending) {
	if o.hooks.Pending != nil {
		o.hooks.Pending(pending)
	}
}

func (o *Orchestrator) emitA2UI(surfaceID string, messages []map[string]any) {
	if o.hooks.A2UI != nil {
		o.hooks.A2UI(surfaceID, messages)
	}
}

func (o *Orchestrator) emitLayout() {
	if o.hooks.Layout != nil {
		o.hooks.Layout(o.registry.Layout())
	}
}

func (o *Orchestrator) emitError(err error) {
	if o.hooks.Error != nil && err != nil {
		o.hooks.Error(err)
	}
}

type httpGetArgs struct {
	URL string `json:"url"`
}

type httpGetResult struct {
	URL         string `json:"url"`
	Status      int    `json:"status"`
	ContentType string `json:"contentType,omitempty"`
	Body        string `json:"body,omitempty"`
	Error       string `json:"error,omitempty"`
}

func newHTTPGetTool(client *http.Client) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "http_get",
		Description: "Fetch weather/geocoding JSON only from api.open-meteo.com or geocoding-api.open-meteo.com over HTTPS.",
	}, func(ctx agent.Context, args httpGetArgs) (httpGetResult, error) {
		parsed, err := url.Parse(strings.TrimSpace(args.URL))
		if err != nil || !allowedHTTPURL(parsed) {
			return httpGetResult{URL: args.URL, Error: "http_get only permits HTTPS api.open-meteo.com and geocoding-api.open-meteo.com"}, nil
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
		if err != nil {
			return httpGetResult{URL: parsed.String(), Error: err.Error()}, nil
		}
		response, err := configuredHTTPClient(client).Do(request)
		if err != nil {
			return httpGetResult{URL: parsed.String(), Error: err.Error()}, nil
		}
		defer response.Body.Close()
		const maxBody = 1 << 20
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
		if readErr != nil {
			return httpGetResult{URL: parsed.String(), Status: response.StatusCode, Error: readErr.Error()}, nil
		}
		if len(body) > maxBody {
			return httpGetResult{URL: parsed.String(), Status: response.StatusCode, Error: "response exceeds 1 MiB limit"}, nil
		}
		return httpGetResult{
			URL:         parsed.String(),
			Status:      response.StatusCode,
			ContentType: response.Header.Get("Content-Type"),
			Body:        string(body),
		}, nil
	})
}

func allowedHTTPURL(value *url.URL) bool {
	if value == nil || value.Scheme != "https" || value.User != nil || value.Port() != "" {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(value.Hostname(), "."))
	return host == "api.open-meteo.com" || host == "geocoding-api.open-meteo.com"
}

func configuredHTTPClient(base *http.Client) *http.Client {
	client := &http.Client{Timeout: 10 * time.Second}
	if base != nil {
		copy := *base
		client = &copy
		if client.Timeout <= 0 {
			client.Timeout = 10 * time.Second
		}
	}
	previousRedirect := client.CheckRedirect
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if !allowedHTTPURL(request.URL) {
			return fmt.Errorf("redirect outside allowed weather hosts")
		}
		if previousRedirect != nil {
			return previousRedirect(request, via)
		}
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	}
	return client
}
