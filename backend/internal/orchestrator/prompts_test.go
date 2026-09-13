package orchestrator

import (
	"errors"
	"strings"
	"testing"

	"voice2canvas/backend/internal/a2ui"
)

func TestBuildRepairPromptContainsOutputAndValidationErrors(t *testing.T) {
	validationText := "message 0: missing required property\nextended component LineChart: y must be number"
	prompt := BuildRepairPrompt("```json\n{broken}\n```", errors.New(validationText))
	for _, want := range []string{
		validationText,
		"{broken}",
		"Return one JSON object only",
		"dataModelPatches",
		"first child of root",
		"5 words or fewer in Title Case",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("repair prompt missing %q: %s", want, prompt)
		}
	}
}

func TestBuildRepairPromptPreservesExtendedSchemaErrorVerbatim(t *testing.T) {
	validator, err := a2ui.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	validationErr := validator.ValidateMessage(map[string]any{
		"version": a2ui.Version,
		"updateComponents": map[string]any{
			"surfaceId": "card_1",
			"components": []any{
				map[string]any{
					"id":        "root",
					"component": "LineChart",
					"series": []any{map[string]any{
						"name":   "Temperature",
						"points": []any{map[string]any{"x": "now", "y": "invalid"}},
					}},
				},
			},
		},
	})
	if validationErr == nil {
		t.Fatal("expected an extended schema validation error")
	}
	prompt := BuildRepairPrompt(`{"components":[]}`, validationErr)
	if !strings.Contains(prompt, validationErr.Error()) {
		t.Fatalf("repair prompt changed extended schema error:\n%s", prompt)
	}
}

func TestBuildCardMessagesPassA2UIValidation(t *testing.T) {
	validator, err := a2ui.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	messages := buildCardMessages("card_1", true, CardEnvelope{
		Title:       "Info",
		Description: "Description",
		DataModel:   map[string]any{"sample": true},
		Components: []map[string]any{
			{"id": "root", "component": "Card", "child": "content"},
			{"id": "content", "component": "Column", "children": []any{"title", "stat", "chart"}},
			{"id": "title", "component": "Text", "text": "Info", "variant": "h3"},
			{"id": "stat", "component": "Stat", "label": "Now", "value": map[string]any{"path": "/current"}},
			{"id": "chart", "component": "LineChart", "series": map[string]any{"path": "/series"}},
		},
	})
	if err := validator.ValidateMessages(messages); err != nil {
		t.Fatalf("generated card messages are invalid: %v", err)
	}
	create := messages[0]["createSurface"].(map[string]any)
	if create["catalogId"] != a2ui.ExtendedCatalogID {
		t.Fatalf("unexpected catalog ID: %v", create["catalogId"])
	}
}

func TestCardAgentInstructionRequiresHeadingTitleForCreateAndUpdate(t *testing.T) {
	for _, want := range []string{
		"Create rules:",
		"Every created card envelope must begin with a title",
		"first child of root must be a heading Text component",
		"5 words or fewer",
		"Title Case",
		"set the envelope title to exactly the same text",
		"Update rules:",
		"Every updated card envelope must still begin with a title",
		`content.children begins ["title","body"]`,
	} {
		if !strings.Contains(cardAgentInstruction, want) {
			t.Errorf("card title guidance missing %q", want)
		}
	}
}

func TestCardAgentAuthoringGuideMentionsEveryExtendedComponent(t *testing.T) {
	for _, component := range []string{
		"Stat", "StatGroup", "LineChart", "BarChart", "Gauge",
		"ProgressBar", "KeyValueList", "Badge", "DataTable",
	} {
		if !strings.Contains(cardAgentInstruction, component) {
			t.Errorf("authoring guide does not mention %s", component)
		}
	}
	for _, guidance := range []string{"at most 24 points", "hourly=temperature_2m", "daily=temperature_2m_max,temperature_2m_min", "sample data", "dataModelPatches"} {
		if !strings.Contains(cardAgentInstruction, guidance) {
			t.Errorf("authoring guide missing %q", guidance)
		}
	}
	for _, guidance := range []string{"Market snapshots", "Weather conditions", "News and general research"} {
		if !strings.Contains(cardAgentInstruction, guidance) {
			t.Errorf("general card guidance missing %q", guidance)
		}
	}
}

func TestSpecialistPromptsAndInstructionsStayGeneralAndSourced(t *testing.T) {
	prompt := specialistPrompt("Market", Task{Intent: IntentCreate, Domain: DomainMarkets, Description: "Compare two indexes"}, nil)
	for _, want := range []string{"Market task intent: create", "Compare two indexes", "configured public data tools"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("specialist prompt missing %q: %s", want, prompt)
		}
	}
	for instruction, wants := range map[string][]string{
		weatherAgentInstruction: {"Open-Meteo", "at most 24", "As of", "Never invent"},
		marketAgentInstruction:  {"Google Search", "percentage change", "timezone", "delayed quotes", "Never provide personalized investment advice"},
	} {
		for _, want := range wants {
			if !strings.Contains(instruction, want) {
				t.Errorf("specialist instruction missing %q", want)
			}
		}
	}
}

func TestUpdatePromptIncludesExistingEnvelopeAndPatchGuidance(t *testing.T) {
	existing := &CardEnvelope{
		Title: "Forecast",
		Components: []map[string]any{
			{"id": "root", "component": "Column", "children": []any{"title", "chart"}},
			{"id": "title", "component": "Text", "text": "Forecast", "variant": "h3"},
			{"id": "chart", "component": "LineChart", "series": map[string]any{"path": "/series"}},
		},
		DataModel: map[string]any{"series": []any{map[string]any{"name": "Temperature"}}},
	}
	prompt := cardPrompt(Task{Intent: IntentUpdate, Description: "refresh", TargetCard: "card_1"}, existing, "")
	for _, want := range []string{"Existing extended-card envelope", `"component":"LineChart"`, `"series"`, "prefer narrow dataModelPatches"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("update prompt missing %q: %s", want, prompt)
		}
	}
}

func TestPatchOnlyUpdateEmitsNarrowMessageAndMaterializesState(t *testing.T) {
	existing := &CardEnvelope{
		Title: "Forecast",
		Components: []map[string]any{
			{"id": "root", "component": "Column", "children": []any{"title", "stat"}},
			{"id": "title", "component": "Text", "text": "Forecast", "variant": "h3"},
			{"id": "stat", "component": "Stat", "label": "Now", "value": map[string]any{"path": "/weather/temperature"}},
		},
		DataModel: map[string]any{"weather": map[string]any{"temperature": 18.0, "condition": "Clear"}},
	}
	generated := CardEnvelope{DataModelPatches: []DataModelPatch{{Path: "/weather/temperature", Value: 21.5}}}
	effective, err := materializeEnvelope(generated, existing, false)
	if err != nil {
		t.Fatal(err)
	}
	weather := effective.DataModel["weather"].(map[string]any)
	if weather["temperature"] != 21.5 || weather["condition"] != "Clear" {
		t.Fatalf("unexpected effective model: %#v", effective.DataModel)
	}
	messages := buildCardMessages("card_1", false, generated)
	if len(messages) != 1 {
		t.Fatalf("patch-only update emitted %d messages: %#v", len(messages), messages)
	}
	update := messages[0]["updateDataModel"].(map[string]any)
	if update["path"] != "/weather/temperature" || update["value"] != 21.5 {
		t.Fatalf("unexpected narrow update: %#v", update)
	}
	validator, err := a2ui.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	if err := validator.ValidateMessages(messages); err != nil {
		t.Fatalf("narrow updateDataModel message is invalid: %v", err)
	}
}

func TestStripJSONFences(t *testing.T) {
	got := stripJSONFences("Here\n```json\n{\"title\":\"x\"}\n```\n")
	if got != `{"title":"x"}` {
		t.Fatalf("unexpected cleaned JSON: %q", got)
	}
}
