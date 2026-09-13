package registry

import (
	"testing"
	"time"
)

func TestRegistryCreateResolveLayoutAndRemove(t *testing.T) {
	registry := New()
	first := registry.Create("One", "First", map[string]any{"value": 1})
	second := registry.Create("Two", "Second", map[string]any{"value": 2})

	if first.SurfaceID != "card_1" || second.SurfaceID != "card_2" {
		t.Fatalf("unexpected surface IDs: %q, %q", first.SurfaceID, second.SurfaceID)
	}
	if first.Order >= second.Order {
		t.Fatalf("orders are not append ordered: %d, %d", first.Order, second.Order)
	}

	resolved, ok := registry.Resolve(first.SurfaceID)
	if !ok || resolved.Title != "One" {
		t.Fatalf("could not resolve first card: %+v, %v", resolved, ok)
	}
	resolved.DataModelSummary["value"] = 99
	unchanged, _ := registry.Resolve(first.SurfaceID)
	if unchanged.DataModelSummary["value"] != float64(1) {
		t.Fatalf("registry leaked mutable data model: %#v", unchanged.DataModelSummary)
	}

	slots := registry.Layout()
	if len(slots) != 2 || slots[0].SurfaceID != "card_1" || slots[1].SurfaceID != "card_2" {
		t.Fatalf("unexpected layout: %+v", slots)
	}
	if _, ok := registry.Remove(first.SurfaceID); !ok {
		t.Fatal("remove did not find first card")
	}
	if _, ok := registry.Resolve(first.SurfaceID); ok {
		t.Fatal("removed card is still present")
	}
}

func TestRegistryCardsReturnsOrderedDeepClonedInventory(t *testing.T) {
	registry := New()
	first := registry.Create("One", "First", map[string]any{"nested": map[string]any{"value": 1}})
	second := registry.Create("Two", "Second", map[string]any{"value": 2})

	cards := registry.Cards()
	if len(cards) != 2 || cards[0].SurfaceID != first.SurfaceID || cards[1].SurfaceID != second.SurfaceID {
		t.Fatalf("unexpected card snapshot: %+v", cards)
	}
	if cards[0].CreatedAt.IsZero() || time.Since(cards[0].CreatedAt) > time.Minute {
		t.Fatalf("unexpected creation timestamp: %v", cards[0].CreatedAt)
	}
	cards[0].DataModelSummary["nested"].(map[string]any)["value"] = 99
	unchanged, _ := registry.Resolve(first.SurfaceID)
	if unchanged.DataModelSummary["nested"].(map[string]any)["value"] != float64(1) {
		t.Fatalf("Cards leaked mutable state: %#v", unchanged.DataModelSummary)
	}
}

func TestRegistryUpsertPreservesOrder(t *testing.T) {
	registry := New()
	created := registry.Create("Old", "Description", map[string]any{"old": true})
	updated, ok := registry.Upsert(created.SurfaceID, "New", "", map[string]any{"new": true})
	if !ok {
		t.Fatal("upsert did not find card")
	}
	if updated.Order != created.Order || updated.Title != "New" || updated.Description != created.Description {
		t.Fatalf("upsert changed unexpected fields: %+v", updated)
	}
}

func TestRegistryRetainsAndClonesComponentState(t *testing.T) {
	registry := New()
	reserved := registry.Reserve()
	components := []map[string]any{{"id": "root", "component": "Stat", "value": map[string]any{"path": "/value"}}}
	created := registry.CommitState(reserved, "Metric", "Current", components, map[string]any{"value": 1})

	created.Components[0]["component"] = "Changed"
	resolved, ok := registry.Resolve(created.SurfaceID)
	if !ok {
		t.Fatal("committed card not found")
	}
	if resolved.Components[0]["component"] != "Stat" {
		t.Fatalf("registry leaked mutable component state: %#v", resolved.Components)
	}
}

func TestRegistryApplyLayoutStoresOrderAndSpan(t *testing.T) {
	registry := New()
	first := registry.Create("One", "First", map[string]any{"value": 1})
	second := registry.Create("Two", "Second", map[string]any{"value": 2})

	if err := registry.ApplyLayout([]Slot{
		{SurfaceID: second.SurfaceID, Order: 1, Span: 2},
		{SurfaceID: first.SurfaceID, Order: 2, Span: 1},
	}); err != nil {
		t.Fatal(err)
	}
	slots := registry.Layout()
	if len(slots) != 2 || slots[0].SurfaceID != second.SurfaceID || slots[0].Span != 2 || slots[1].SurfaceID != first.SurfaceID {
		t.Fatalf("curated layout was not retained: %+v", slots)
	}
	resolved, _ := registry.Resolve(second.SurfaceID)
	if resolved.Order != 1 || resolved.Span != 2 {
		t.Fatalf("curated card placement was not retained: %+v", resolved)
	}
}

func TestRegistryApplyLayoutRejectsInvalidLayout(t *testing.T) {
	registry := New()
	first := registry.Create("One", "First", nil)
	second := registry.Create("Two", "Second", nil)

	for name, slots := range map[string][]Slot{
		"duplicate order": {{SurfaceID: first.SurfaceID, Order: 1, Span: 1}, {SurfaceID: second.SurfaceID, Order: 1, Span: 2}},
		"invalid span":    {{SurfaceID: first.SurfaceID, Order: 1, Span: 3}, {SurfaceID: second.SurfaceID, Order: 2, Span: 1}},
		"unknown card":    {{SurfaceID: "surface-missing", Order: 1, Span: 1}},
		"more slots than cards": {
			{SurfaceID: first.SurfaceID, Order: 1, Span: 1},
			{SurfaceID: second.SurfaceID, Order: 2, Span: 1},
			{SurfaceID: "surface-extra", Order: 3, Span: 1},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := registry.ApplyLayout(slots); err == nil {
				t.Fatal("invalid layout was accepted")
			}
		})
	}
	current := registry.Layout()
	if current[0].SurfaceID != first.SurfaceID || current[0].Span != 1 || current[1].SurfaceID != second.SurfaceID {
		t.Fatalf("invalid layout changed registry state: %+v", current)
	}
}

// A card that finishes generating while the curator is running is not in the
// curated slots; it must be appended rather than failing the whole arrange.
func TestRegistryApplyLayoutAppendsCardsAddedDuringCuration(t *testing.T) {
	registry := New()
	first := registry.Create("One", "First", nil)
	second := registry.Create("Two", "Second", nil)

	// The curator saw only these two cards.
	curated := []Slot{
		{SurfaceID: second.SurfaceID, Order: 1, Span: 2},
		{SurfaceID: first.SurfaceID, Order: 2, Span: 1},
	}
	// Two more land before the layout is applied.
	third := registry.Create("Three", "Third", nil)
	fourth := registry.Create("Four", "Fourth", nil)

	if err := registry.ApplyLayout(curated); err != nil {
		t.Fatalf("layout rejected after concurrent card creation: %v", err)
	}

	slots := registry.Layout()
	if len(slots) != 4 {
		t.Fatalf("expected every live card in the layout, got %+v", slots)
	}
	want := []string{second.SurfaceID, first.SurfaceID, third.SurfaceID, fourth.SurfaceID}
	for index, surfaceID := range want {
		if slots[index].SurfaceID != surfaceID {
			t.Fatalf("slot %d: want %s, got %s (%+v)", index, surfaceID, slots[index].SurfaceID, slots)
		}
	}
	if slots[0].Span != 2 {
		t.Fatalf("curated span was lost: %+v", slots[0])
	}

	// Later cards must not collide with the orders just assigned.
	fifth := registry.Create("Five", "Fifth", nil)
	resolved, _ := registry.Resolve(fifth.SurfaceID)
	if resolved.Order <= slots[3].Order {
		t.Fatalf("nextOrder not advanced past appended cards: %+v", resolved)
	}
}
