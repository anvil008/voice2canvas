package orchestrator

import (
	"context"
	"errors"
	"iter"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"voice2canvas/backend/internal/registry"
)

type fakeLLM struct {
	mu       sync.Mutex
	response string
	usage    *genai.GenerateContentResponseUsageMetadata
	results  []fakeLLMResult
	calls    int
	prompts  []string
}

type fakeLLMResult struct {
	response string
	usage    *genai.GenerateContentResponseUsageMetadata
	err      error
}

func (f *fakeLLM) Name() string { return "fake-card-model" }

func (f *fakeLLM) GenerateContent(_ context.Context, request *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	f.mu.Lock()
	for _, content := range request.Contents {
		for _, part := range content.Parts {
			if part != nil && part.Text != "" {
				f.prompts = append(f.prompts, part.Text)
			}
		}
	}
	result := fakeLLMResult{response: f.response, usage: f.usage}
	if f.calls < len(f.results) {
		result = f.results[f.calls]
	}
	f.calls++
	f.mu.Unlock()
	return func(yield func(*model.LLMResponse, error) bool) {
		if result.err != nil {
			yield(nil, result.err)
			return
		}
		yield(&model.LLMResponse{
			Content:       genai.NewContentFromText(result.response, genai.RoleModel),
			UsageMetadata: result.usage,
		}, nil)
	}
}

func (f *fakeLLM) allPrompts() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.prompts, "\n")
}

func waitForRendered(t *testing.T, statuses <-chan TaskStatus, taskID string) TaskStatus {
	t.Helper()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case status := <-statuses:
			if status.TaskID == taskID && (status.Status == statusRendered || status.Status == statusFailed) {
				return status
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for task %s", taskID)
		}
	}
}

func TestCreateDispatchEmitsPendingBeforeGeneration(t *testing.T) {
	model := &fakeLLM{response: `{"title":"Wrong Metadata","description":"Current weather","components":[{"id":"root","component":"Column","children":["title","temperature"]},{"id":"title","component":"Text","text":"Denver Weather","variant":"h3"},{"id":"temperature","component":"Stat","label":"Temperature","value":{"path":"/temperature"}}],"dataModel":{"temperature":20}}`}
	pending := make(chan CardPending, 1)
	statuses := make(chan TaskStatus, 8)
	orch, err := New(Config{CardModel: model, Hooks: Hooks{
		Pending: func(value CardPending) { pending <- value },
		Status:  func(value TaskStatus) { statuses <- value },
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	taskID := orch.Dispatch(Task{Intent: IntentCreate, Description: "Show Denver weather right now"})
	select {
	case value := <-pending:
		if value.TaskID != taskID || value.SurfaceID != "card_1" || !strings.Contains(value.Title, "Denver weather") {
			t.Fatalf("unexpected pending card: %+v", value)
		}
	default:
		t.Fatal("create Dispatch did not emit card_pending synchronously")
	}
	if status := waitForRendered(t, statuses, taskID); status.Status != statusRendered {
		t.Fatalf("create failed: %+v", status)
	}
	card, ok := orch.registry.Resolve("card_1")
	if !ok || card.Title != "Denver Weather" {
		t.Fatalf("registry title was not derived from rendered heading: %+v", card)
	}
}

func TestWorkerUsageHookSumsADKMetadata(t *testing.T) {
	model := &fakeLLM{
		response: "done",
		usage: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     13,
			CandidatesTokenCount: 5,
			ThoughtsTokenCount:   2,
		},
	}
	usage := make(chan [3]any, 1)
	orch, err := New(Config{CardModel: model, Hooks: Hooks{
		Usage: func(inputTokens, outputTokens int, source string) {
			usage <- [3]any{inputTokens, outputTokens, source}
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()
	if _, err := orch.runCardAgent("test usage"); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-usage:
		if got != [3]any{13, 7, "worker"} {
			t.Fatalf("unexpected usage hook values: %#v", got)
		}
	default:
		t.Fatal("worker usage hook did not fire")
	}
}

func TestResearchPipelineOrdersStatusesAndInjectsBrief(t *testing.T) {
	brief := "As of: 2026-08-02 00:00 UTC\nLatest close: ACME $42.10\nSource: Example Exchange"
	model := &fakeLLM{results: []fakeLLMResult{
		{response: brief},
		{response: `{"title":"ACME Close","description":"Latest close","components":[{"id":"root","component":"Card","child":"content"},{"id":"content","component":"Column","children":["title","value","source"]},{"id":"title","component":"Text","text":"ACME Close","variant":"h3"},{"id":"value","component":"Stat","label":"Close","value":{"path":"/close"},"unit":"$"},{"id":"source","component":"Text","text":"Source: Example Exchange","variant":"caption"}],"dataModel":{"close":42.1}}`},
	}}
	statuses := make(chan TaskStatus, 12)
	orch, err := New(Config{CardModel: model, Concurrency: 1, Hooks: Hooks{Status: func(value TaskStatus) { statuses <- value }}})
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	taskID := orch.Dispatch(Task{Intent: IntentCreate, Description: "Create a card with the latest ACME share price"})
	var sequence []TaskStatus
	for {
		select {
		case status := <-statuses:
			if status.TaskID != taskID {
				continue
			}
			sequence = append(sequence, status)
			if status.Status == statusRendered || status.Status == statusFailed {
				goto complete
			}
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for researched card")
		}
	}

complete:
	var researchingAt, generatingAt = -1, -1
	for index, status := range sequence {
		if status.Status == statusResearching && researchingAt < 0 {
			researchingAt = index
		}
		if status.Status == statusGenerating && generatingAt < 0 {
			generatingAt = index
		}
	}
	if researchingAt < 0 || generatingAt < 0 || researchingAt >= generatingAt {
		t.Fatalf("researching did not precede generating: %+v", sequence)
	}
	prompts := model.allPrompts()
	for _, want := range []string{"RESEARCH DATA BRIEF (authoritative", "ACME $42.10", "cite its source names"} {
		if !strings.Contains(prompts, want) {
			t.Errorf("card prompt missing research brief guidance %q: %s", want, prompts)
		}
	}
}

func TestResearchFailureFallsBackToCardGeneration(t *testing.T) {
	model := &fakeLLM{results: []fakeLLMResult{
		{err: errors.New("search backend unavailable")},
		{response: `{"title":"ACME Sample","description":"Fallback data","components":[{"id":"root","component":"Card","child":"content"},{"id":"content","component":"Column","children":["title","source"]},{"id":"title","component":"Text","text":"ACME Sample","variant":"h3"},{"id":"source","component":"Text","text":"sample data","variant":"caption"}],"dataModel":{"sample":true}}`},
	}}
	statuses := make(chan TaskStatus, 12)
	orch, err := New(Config{CardModel: model, Concurrency: 1, Hooks: Hooks{Status: func(value TaskStatus) { statuses <- value }}})
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	taskID := orch.Dispatch(Task{Intent: IntentCreate, Description: "Show the latest ACME market value"})
	var sawResearchFailureDetail bool
	for {
		select {
		case status := <-statuses:
			if status.TaskID != taskID {
				continue
			}
			if status.Status == statusResearching && strings.Contains(status.Detail, "continuing with card generation") {
				sawResearchFailureDetail = true
			}
			if status.Status == statusRendered {
				if !sawResearchFailureDetail {
					t.Fatal("research failure detail was not emitted before fallback generation")
				}
				if _, ok := orch.Registry().Resolve("card_1"); !ok {
					t.Fatal("fallback card generation did not commit a card")
				}
				return
			}
			if status.Status == statusFailed {
				t.Fatalf("research failure hard-failed the card: %+v", status)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for researcher fallback")
		}
	}
}

func TestMarketCreateRoutesThroughSpecialistAndInjectsBrief(t *testing.T) {
	brief := "ACME: $125.20, +2.4%\nSession: open\nSource: Example Exchange\nAs of: 2026-08-27 18:00 UTC"
	model := &fakeLLM{results: []fakeLLMResult{
		{response: brief},
		{response: `{"title":"ACME Market","description":"Current market snapshot","components":[{"id":"root","component":"Card","child":"content"},{"id":"content","component":"Column","children":["title","price","source"]},{"id":"title","component":"Text","text":"ACME Market","variant":"h3"},{"id":"price","component":"Stat","label":"Price","value":{"path":"/price"},"unit":"USD","delta":2.4},{"id":"source","component":"Text","text":"Source: Example Exchange","variant":"caption"}],"dataModel":{"price":125.2}}`},
	}}
	statuses := make(chan TaskStatus, 12)
	orch, err := New(Config{CardModel: model, Concurrency: 1, Hooks: Hooks{Status: func(value TaskStatus) { statuses <- value }}})
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	taskID := orch.Dispatch(Task{Intent: IntentCreate, Domain: DomainMarkets, Description: "Show ACME market data"})
	var sequence []TaskStatus
	for {
		select {
		case status := <-statuses:
			if status.TaskID != taskID {
				continue
			}
			sequence = append(sequence, status)
			if status.Status == statusRendered || status.Status == statusFailed {
				goto complete
			}
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for market card")
		}
	}

complete:
	if sequence[len(sequence)-1].Status != statusRendered {
		t.Fatalf("market card did not render: %+v", sequence)
	}
	var researchingAt, generatingAt = -1, -1
	for index, status := range sequence {
		if status.Status == statusResearching && researchingAt < 0 {
			researchingAt = index
		}
		if status.Status == statusGenerating && generatingAt < 0 {
			generatingAt = index
		}
	}
	if researchingAt < 0 || generatingAt < 0 || researchingAt >= generatingAt {
		t.Fatalf("market researching did not precede generating: %+v", sequence)
	}
	prompts := model.allPrompts()
	for _, want := range []string{"Market task intent: create", "ACME: $125.20", "RESEARCH DATA BRIEF (authoritative"} {
		if !strings.Contains(prompts, want) {
			t.Errorf("market/card prompt missing %q: %s", want, prompts)
		}
	}
	if strings.Contains(prompts, "Dashboard-card task intent:") {
		t.Fatalf("market task incorrectly used the general researcher prompt: %s", prompts)
	}
}

func TestWeatherInvestigationCollectsBriefBeforeSpokenFinding(t *testing.T) {
	brief := "Denver: 18°C and clear, wind 11 km/h\nAs of: 2026-08-27 18:00 UTC"
	finding := "Denver is currently clear at 18 degrees Celsius, with light wind around 11 kilometers per hour."
	model := &fakeLLM{results: []fakeLLMResult{{response: brief}, {response: finding}}}
	statuses := make(chan TaskStatus, 12)
	delivered := make(chan string, 1)
	orch, err := New(Config{
		CardModel: model, Concurrency: 1,
		DeliverFinding: func(text string) error { delivered <- text; return nil },
		Hooks:          Hooks{Status: func(value TaskStatus) { statuses <- value }},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	taskID := orch.Dispatch(Task{Intent: IntentInvestigate, Domain: DomainWeather, Description: "How is the weather in Denver?"})
	status := waitForRendered(t, statuses, taskID)
	if status.Status != statusRendered || !strings.Contains(status.Detail, "currently clear") {
		t.Fatalf("unexpected weather investigation: %+v", status)
	}
	select {
	case text := <-delivered:
		if !strings.Contains(text, finding) {
			t.Fatalf("weather finding did not use normal voice delivery: %q", text)
		}
	case <-time.After(time.Second):
		t.Fatal("weather investigation was not delivered")
	}
	prompts := model.allPrompts()
	for _, want := range []string{"Weather task intent: investigate", "SPECIALIST DATA BRIEF (authoritative)", "Denver: 18°C"} {
		if !strings.Contains(prompts, want) {
			t.Errorf("weather investigation prompt missing %q: %s", want, prompts)
		}
	}
}

func TestPatchOnlyUpdateFoldsStoredDataModelWithoutPending(t *testing.T) {
	cardRegistry := registry.New()
	reserved := cardRegistry.Reserve()
	cardRegistry.CommitState(reserved, "Forecast", "Today", []map[string]any{
		{"id": "root", "component": "Column", "children": []any{"title", "stat"}},
		{"id": "title", "component": "Text", "text": "Denver Forecast", "variant": "h3"},
		{"id": "stat", "component": "Stat", "label": "Now", "value": map[string]any{"path": "/weather/temperature"}},
	}, map[string]any{"weather": map[string]any{"temperature": 18.0, "condition": "Clear"}})
	model := &fakeLLM{response: `{"dataModelPatches":[{"path":"/weather/temperature","value":21.5}]}`}
	statuses := make(chan TaskStatus, 8)
	pending := make(chan CardPending, 1)
	orch, err := New(Config{CardModel: model, Registry: cardRegistry, Hooks: Hooks{
		Status:  func(value TaskStatus) { statuses <- value },
		Pending: func(value CardPending) { pending <- value },
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	taskID := orch.Dispatch(Task{Intent: IntentUpdate, Description: "Refresh the temperature", TargetCard: reserved.SurfaceID})
	if status := waitForRendered(t, statuses, taskID); status.Status != statusRendered {
		t.Fatalf("update failed: %+v", status)
	}
	updated, _ := cardRegistry.Resolve(reserved.SurfaceID)
	weather := updated.DataModelSummary["weather"].(map[string]any)
	if weather["temperature"] != 21.5 || weather["condition"] != "Clear" {
		t.Fatalf("stored data model did not fold patch: %#v", updated.DataModelSummary)
	}
	if updated.Title != "Denver Forecast" {
		t.Fatalf("patch update did not reconcile registry title with heading: %q", updated.Title)
	}
	select {
	case value := <-pending:
		t.Fatalf("narrow patch update emitted card_pending: %+v", value)
	default:
	}
}

func TestWholesaleUpdateEmitsPending(t *testing.T) {
	cardRegistry := registry.New()
	card := cardRegistry.Create("Metric", "Old", map[string]any{"value": 1})
	model := &fakeLLM{response: `{"title":"Metric Chart","components":[{"id":"root","component":"Column","children":["title","chart"]},{"id":"title","component":"Text","text":"Metric Chart","variant":"h3"},{"id":"chart","component":"BarChart","categories":["A"],"series":[{"name":"Value","values":[2]}]}],"dataModel":{"value":2}}`}
	statuses := make(chan TaskStatus, 8)
	pending := make(chan CardPending, 1)
	orch, err := New(Config{CardModel: model, Registry: cardRegistry, Hooks: Hooks{
		Status:  func(value TaskStatus) { statuses <- value },
		Pending: func(value CardPending) { pending <- value },
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	taskID := orch.Dispatch(Task{Intent: IntentUpdate, Description: "Turn it into a comparison", TargetCard: card.SurfaceID})
	if status := waitForRendered(t, statuses, taskID); status.Status != statusRendered {
		t.Fatalf("update failed: %+v", status)
	}
	select {
	case value := <-pending:
		if value.SurfaceID != card.SurfaceID || value.TaskID != taskID || value.Title != "Metric Chart" {
			t.Fatalf("unexpected wholesale pending card: %+v", value)
		}
	default:
		t.Fatal("wholesale update did not emit card_pending")
	}
}

func TestInvestigateRoutesToAnalystWithCardDataAndDeliversFinding(t *testing.T) {
	cardRegistry := registry.New()
	card := cardRegistry.Create("Weather", "Denver conditions", map[string]any{"temperature": 23.5, "humidity": 31})
	model := &fakeLLM{response: "The temperature is higher because dry air and sunshine warmed Denver quickly. Humidity remains low, which supports that explanation."}
	statuses := make(chan TaskStatus, 8)
	delivered := make(chan string, 1)
	orch, err := New(Config{
		CardModel: model, Registry: cardRegistry,
		DeliverFinding: func(text string) error { delivered <- text; return nil },
		Hooks:          Hooks{Status: func(value TaskStatus) { statuses <- value }},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	taskID := orch.Dispatch(Task{Intent: IntentInvestigate, Description: "Why is the temperature so high?", TargetCard: card.SurfaceID})
	status := waitForRendered(t, statuses, taskID)
	if status.Status != statusRendered || !strings.Contains(status.Detail, "temperature is higher") {
		t.Fatalf("unexpected investigation status: %+v", status)
	}
	select {
	case text := <-delivered:
		if !strings.HasPrefix(text, "[investigation result — relay to the user conversationally, briefly] ") {
			t.Fatalf("unexpected finding delivery shape: %q", text)
		}
	case <-time.After(time.Second):
		t.Fatal("investigation finding was not delivered")
	}
	prompt := model.allPrompts()
	for _, want := range []string{`"surfaceId":"` + card.SurfaceID + `"`, `"temperature":23.5`, "Why is the temperature so high?"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("investigator prompt missing %q: %s", want, prompt)
		}
	}
}

func TestArrangeRepairsCuratorJSONAppliesLayoutAndDeliversNote(t *testing.T) {
	cardRegistry := registry.New()
	firstReserved := cardRegistry.Reserve()
	first := cardRegistry.CommitState(firstReserved, "Revenue", "Quarterly trend", []map[string]any{
		{"id": "root", "component": "LineChart", "series": []any{}},
	}, map[string]any{"series": []any{}})
	secondReserved := cardRegistry.Reserve()
	second := cardRegistry.CommitState(secondReserved, "Total", "Headline metric", []map[string]any{
		{"id": "root", "component": "Stat", "label": "Total", "value": 42},
	}, map[string]any{"value": 42})
	model := &fakeLLM{results: []fakeLLMResult{
		{response: `{"slots":[{"surfaceId":"card_1","order":1,"span":3},{"surfaceId":"card_2","order":1,"span":1}],"note":"Done."}`},
		{response: `{"slots":[{"surfaceId":"card_2","order":1,"span":1},{"surfaceId":"card_1","order":2,"span":2}],"note":"Done — the total is first and the revenue chart is wider."}`},
	}}
	statuses := make(chan TaskStatus, 8)
	layouts := make(chan []registry.Slot, 2)
	delivered := make(chan string, 1)
	orch, err := New(Config{
		CardModel: model, Registry: cardRegistry, Concurrency: 1,
		DeliverFinding: func(text string) error { delivered <- text; return nil },
		Hooks: Hooks{
			Status: func(value TaskStatus) { statuses <- value },
			Layout: func(value []registry.Slot) { layouts <- value },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	taskID := orch.Dispatch(Task{Intent: IntentArrange, Description: "Put the total above revenue and make the revenue chart bigger", TargetCard: second.SurfaceID})
	status := waitForRendered(t, statuses, taskID)
	if status.Status != statusRendered || !strings.Contains(status.Detail, "revenue chart is wider") {
		t.Fatalf("arrangement did not render after repair: %+v", status)
	}
	select {
	case slots := <-layouts:
		if len(slots) != 2 || slots[0].SurfaceID != second.SurfaceID || slots[0].Span != 1 || slots[1].SurfaceID != first.SurfaceID || slots[1].Span != 2 {
			t.Fatalf("unexpected applied layout: %+v", slots)
		}
	default:
		t.Fatal("arrangement did not emit a layout")
	}
	select {
	case extra := <-layouts:
		t.Fatalf("successful arrangement emitted more than one layout: %+v", extra)
	default:
	}
	select {
	case text := <-delivered:
		if !strings.HasPrefix(text, "[layout result — relay to the user conversationally, briefly] ") {
			t.Fatalf("unexpected layout note delivery: %q", text)
		}
	case <-time.After(time.Second):
		t.Fatal("layout note was not delivered to the live session")
	}
	prompts := model.allPrompts()
	for _, want := range []string{"componentKinds", `"chart"`, `"stat"`, "span must be 1 or 2", "Required live cards"} {
		if !strings.Contains(prompts, want) {
			t.Errorf("curator prompt/repair missing %q: %s", want, prompts)
		}
	}
}

func TestArrangeFallsBackToCurrentLayoutAfterFailedRepair(t *testing.T) {
	cardRegistry := registry.New()
	first := cardRegistry.Create("One", "First", nil)
	second := cardRegistry.Create("Two", "Second", nil)
	model := &fakeLLM{results: []fakeLLMResult{
		{response: `{"slots":[],"note":"No."}`},
		{response: `{"slots":[{"surfaceId":"card_1","order":1,"span":9}],"note":"Still invalid."}`},
	}}
	statuses := make(chan TaskStatus, 8)
	layouts := make(chan []registry.Slot, 1)
	orch, err := New(Config{CardModel: model, Registry: cardRegistry, Concurrency: 1, Hooks: Hooks{
		Status: func(value TaskStatus) { statuses <- value },
		Layout: func(value []registry.Slot) { layouts <- value },
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	taskID := orch.Dispatch(Task{Intent: IntentArrange, Description: "Tidy the cards"})
	status := waitForRendered(t, statuses, taskID)
	if status.Status != statusFailed || !strings.Contains(status.Detail, "curator repair failed") {
		t.Fatalf("invalid repaired layout did not fail safely: %+v", status)
	}
	select {
	case slots := <-layouts:
		if len(slots) != 2 || slots[0].SurfaceID != first.SurfaceID || slots[0].Span != 1 || slots[1].SurfaceID != second.SurfaceID || slots[1].Span != 1 {
			t.Fatalf("fallback did not preserve current layout: %+v", slots)
		}
	case <-time.After(time.Second):
		t.Fatal("failed arrangement did not emit fallback layout")
	}
}

type fakeRefreshTicker struct {
	ch      chan time.Time
	stopped chan struct{}
	once    sync.Once
}

func newFakeRefreshTicker() *fakeRefreshTicker {
	return &fakeRefreshTicker{ch: make(chan time.Time), stopped: make(chan struct{})}
}

func (f *fakeRefreshTicker) C() <-chan time.Time { return f.ch }
func (f *fakeRefreshTicker) Stop() {
	f.once.Do(func() { close(f.stopped) })
}

func waitTickerStopped(t *testing.T, ticker *fakeRefreshTicker) {
	t.Helper()
	select {
	case <-ticker.stopped:
	case <-time.After(time.Second):
		t.Fatal("refresh ticker was not stopped")
	}
}

func TestRefreshTickerLifecycleReplaceRemoveAndClose(t *testing.T) {
	cardRegistry := registry.New()
	card := cardRegistry.Create("Weather", "Denver", map[string]any{"temperature": 20})
	model := &fakeLLM{response: `{"dataModelPatches":[{"path":"/temperature","value":21}]}`}
	var factoryMu sync.Mutex
	var tickers []*fakeRefreshTicker
	var intervals []time.Duration
	orch, err := New(Config{
		CardModel: model, Registry: cardRegistry,
		tickerFactory: func(interval time.Duration) refreshTicker {
			ticker := newFakeRefreshTicker()
			factoryMu.Lock()
			tickers = append(tickers, ticker)
			intervals = append(intervals, interval)
			factoryMu.Unlock()
			return ticker
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	orch.startRefresher(card.SurfaceID, "first", "", 30)
	orch.startRefresher(card.SurfaceID, "replacement", "", 60)
	factoryMu.Lock()
	if len(tickers) != 2 || intervals[0] != 30*time.Second || intervals[1] != 60*time.Second {
		factoryMu.Unlock()
		t.Fatalf("unexpected refresh factory calls: %v, %v", len(tickers), intervals)
	}
	first, replacement := tickers[0], tickers[1]
	factoryMu.Unlock()
	waitTickerStopped(t, first)

	orch.remove(Task{ID: "remove", Intent: IntentRemove, TargetCard: card.SurfaceID})
	waitTickerStopped(t, replacement)
	orch.refreshes.mu.Lock()
	remaining := len(orch.refreshes.refreshers)
	orch.refreshes.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("refresh loop survived card removal: %d", remaining)
	}

	secondCard := cardRegistry.Create("Other", "Other", map[string]any{"value": 1})
	orch.startRefresher(secondCard.SurfaceID, "close", "", 30)
	factoryMu.Lock()
	closing := tickers[2]
	factoryMu.Unlock()
	orch.Close()
	waitTickerStopped(t, closing)
}

func TestRefreshCapStopsOldestLoopWithStatusNote(t *testing.T) {
	cardRegistry := registry.New()
	model := &fakeLLM{response: `{"dataModelPatches":[{"path":"/value","value":2}]}`}
	statuses := make(chan TaskStatus, 16)
	var tickers []*fakeRefreshTicker
	orch, err := New(Config{
		CardModel: model, Registry: cardRegistry,
		Hooks: Hooks{Status: func(value TaskStatus) { statuses <- value }},
		tickerFactory: func(time.Duration) refreshTicker {
			ticker := newFakeRefreshTicker()
			tickers = append(tickers, ticker)
			return ticker
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	var cards []registry.Card
	for index := 0; index < 5; index++ {
		cards = append(cards, cardRegistry.Create("Card", "Refresh", map[string]any{"value": index}))
		orch.startRefresher(cards[index].SurfaceID, "refresh-task-"+string(rune('1'+index)), "", 30)
	}
	waitTickerStopped(t, tickers[0])
	orch.refreshes.mu.Lock()
	remaining := len(orch.refreshes.refreshers)
	_, oldestStillPresent := orch.refreshes.refreshers[cards[0].SurfaceID]
	orch.refreshes.mu.Unlock()
	if remaining != maxRefreshingCards || oldestStillPresent {
		t.Fatalf("refresh cap was not enforced: remaining=%d oldestPresent=%v", remaining, oldestStillPresent)
	}
	select {
	case status := <-statuses:
		if status.SurfaceID != cards[0].SurfaceID || !strings.Contains(status.Detail, "four-card refresh limit") {
			t.Fatalf("unexpected refresh-cap status: %+v", status)
		}
	case <-time.After(time.Second):
		t.Fatal("refresh cap did not emit a task_status note")
	}
}

func TestSharedRefreshCoordinatorStopsLoopOnOtherOrchestratorRemoval(t *testing.T) {
	cardRegistry := registry.New()
	card := cardRegistry.Create("Shared", "Across clients", map[string]any{"value": 1})
	refreshes := NewRefreshCoordinator()
	ticker := newFakeRefreshTicker()
	first, err := New(Config{
		CardModel: &fakeLLM{response: `{"dataModelPatches":[{"path":"/value","value":2}]}`},
		Registry:  cardRegistry, Refreshes: refreshes,
		tickerFactory: func(time.Duration) refreshTicker { return ticker },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := New(Config{
		CardModel: &fakeLLM{response: `{"dataModelPatches":[{"path":"/value","value":3}]}`},
		Registry:  cardRegistry, Refreshes: refreshes,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	first.startRefresher(card.SurfaceID, "from-first-client", "", 30)
	second.remove(Task{ID: "from-second-client", Intent: IntentRemove, TargetCard: card.SurfaceID})
	waitTickerStopped(t, ticker)
	refreshes.mu.Lock()
	remaining := len(refreshes.refreshers)
	refreshes.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("shared refresh survived cross-orchestrator removal: %d", remaining)
	}
}
