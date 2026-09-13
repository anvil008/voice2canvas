package a2ui

import "testing"

func TestValidatorAcceptsBasicCardMessages(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatal(err)
	}

	messages := []map[string]any{
		{
			"version": Version,
			"createSurface": map[string]any{
				"surfaceId":     "card_1",
				"catalogId":     BasicCatalogID,
				"sendDataModel": true,
			},
		},
		{
			"version": Version,
			"updateComponents": map[string]any{
				"surfaceId": "card_1",
				"components": []any{
					map[string]any{"id": "root", "component": "Text", "text": "Hello", "variant": "body"},
				},
			},
		},
	}
	if err := validator.ValidateMessages(messages); err != nil {
		t.Fatalf("valid card rejected: %v", err)
	}
}

func TestValidatorRejectsBrokenCard(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatal(err)
	}

	err = validator.ValidateMessage(map[string]any{
		"version": Version,
		"updateComponents": map[string]any{
			"surfaceId": "card_1",
			"components": []any{
				map[string]any{"id": "not-root", "component": "NotInTheBasicCatalog"},
			},
		},
	})
	if err == nil {
		t.Fatal("broken card unexpectedly passed validation")
	}
}

func TestValidatorAcceptsExtendedLineChart(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatal(err)
	}

	messages := []map[string]any{
		{
			"version": Version,
			"createSurface": map[string]any{
				"surfaceId": "card_1",
				"catalogId": ExtendedCatalogID,
			},
		},
		lineChartMessage([]any{
			map[string]any{
				"name": "Temperature",
				"points": []any{
					map[string]any{"x": "2026-08-02T00:00:00Z", "y": 18.5},
					map[string]any{"x": "2026-08-02T01:00:00Z", "y": 18.0},
				},
			},
		}),
	}
	if err := validator.ValidateMessages(messages); err != nil {
		t.Fatalf("valid extended LineChart rejected: %v", err)
	}
}

func TestValidatorRejectsLineChartWithStringY(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatal(err)
	}

	err = validator.ValidateMessage(lineChartMessage([]any{
		map[string]any{
			"name": "Temperature",
			"points": []any{
				map[string]any{"x": "2026-08-02T00:00:00Z", "y": "hot"},
			},
		},
	}))
	if err == nil {
		t.Fatal("LineChart with string y unexpectedly passed validation")
	}
}

func TestValidatorRejectsUnknownExtendedComponent(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatal(err)
	}

	err = validator.ValidateMessage(map[string]any{
		"version": Version,
		"updateComponents": map[string]any{
			"surfaceId": "card_1",
			"components": []any{
				map[string]any{"id": "root", "component": "PieChart", "series": []any{}},
			},
		},
	})
	if err == nil {
		t.Fatal("unknown component unexpectedly passed validation")
	}
}

func lineChartMessage(series []any) map[string]any {
	return map[string]any{
		"version": Version,
		"updateComponents": map[string]any{
			"surfaceId": "card_1",
			"components": []any{
				map[string]any{
					"id":        "root",
					"component": "LineChart",
					"series":    series,
					"xType":     "time",
				},
			},
		},
	}
}
