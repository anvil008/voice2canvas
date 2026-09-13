// Package registry tracks the process-global dashboard card state.
package registry

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Card is the server-side state used to resolve updates/removals, provide
// update context to the card agent, and build the append-order layout frame.
type Card struct {
	SurfaceID        string
	Title            string
	Description      string
	Components       []map[string]any
	DataModelSummary map[string]any
	CreatedAt        time.Time
	Order            int
	Span             int
}

// Slot is the wire representation used inside a layout frame.
type Slot struct {
	SurfaceID string `json:"surfaceId"`
	Order     int    `json:"order"`
	Span      int    `json:"span"`
}

// Registry is safe for use by concurrent task goroutines.
type Registry struct {
	mu        sync.RWMutex
	cards     map[string]Card
	nextID    int
	nextOrder int
}

func New() *Registry {
	return &Registry{cards: make(map[string]Card), nextID: 1, nextOrder: 1}
}

// Create allocates a server-owned card_<n> surface and inserts it.
func (r *Registry) Create(title, description string, dataModel map[string]any) Card {
	reserved := r.Reserve()
	return r.Commit(reserved, title, description, dataModel)
}

// Reserve allocates an ID and append order without making the card visible in
// Layout. A task can therefore generate concurrently without exposing slots
// for cards that have not rendered yet.
func (r *Registry) Reserve() Card {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cards == nil {
		r.cards = make(map[string]Card)
	}
	card := Card{
		SurfaceID: "card_" + itoa(r.nextID),
		CreatedAt: time.Now().UTC(),
		Order:     r.nextOrder,
		Span:      1,
	}
	r.nextID++
	r.nextOrder++
	return cloneCard(card)
}

// Commit makes a previously reserved card visible in the registry.
func (r *Registry) Commit(reserved Card, title, description string, dataModel map[string]any) Card {
	return r.CommitState(reserved, title, description, nil, dataModel)
}

// CommitState makes a reserved card visible and retains its complete rendered
// component/data state for later model-driven updates.
func (r *Registry) CommitState(reserved Card, title, description string, components []map[string]any, dataModel map[string]any) Card {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cards == nil {
		r.cards = make(map[string]Card)
	}
	reserved.Title = title
	reserved.Description = description
	reserved.Components = cloneComponents(components)
	reserved.DataModelSummary = cloneMap(dataModel)
	r.cards[reserved.SurfaceID] = reserved
	return cloneCard(reserved)
}

// Upsert updates a known card while preserving its surface ID and order. The
// bool is false when surfaceID was not present.
func (r *Registry) Upsert(surfaceID, title, description string, dataModel map[string]any) (Card, bool) {
	return r.UpsertState(surfaceID, title, description, nil, dataModel)
}

// UpsertState replaces the effective rendered state while preserving the
// surface ID and layout order. A nil components slice preserves the tree.
func (r *Registry) UpsertState(surfaceID, title, description string, components []map[string]any, dataModel map[string]any) (Card, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	card, ok := r.cards[surfaceID]
	if !ok {
		return Card{}, false
	}
	if title != "" {
		card.Title = title
	}
	if description != "" {
		card.Description = description
	}
	if components != nil {
		card.Components = cloneComponents(components)
	}
	card.DataModelSummary = cloneMap(dataModel)
	r.cards[surfaceID] = card
	return cloneCard(card), true
}

func (r *Registry) Resolve(surfaceID string) (Card, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	card, ok := r.cards[surfaceID]
	if !ok {
		return Card{}, false
	}
	return cloneCard(card), true
}

func (r *Registry) Remove(surfaceID string) (Card, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	card, ok := r.cards[surfaceID]
	if !ok {
		return Card{}, false
	}
	delete(r.cards, surfaceID)
	return cloneCard(card), true
}

func (r *Registry) Layout() []Slot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	slots := make([]Slot, 0, len(r.cards))
	for _, card := range r.cards {
		slots = append(slots, Slot{SurfaceID: card.SurfaceID, Order: card.Order, Span: card.Span})
	}
	sort.Slice(slots, func(i, j int) bool {
		return slots[i].Order < slots[j].Order
	})
	return slots
}

// ApplyLayout replaces the order and span of the cards the curator addressed.
// Curation takes seconds, so cards can finish generating while it runs; those
// are appended after the curated block in their existing relative order rather
// than failing the arrange. Malformed output — unknown cards, repeats, bad
// orders or spans — is still rejected, so a stale layout cannot silently drop
// a card.
func (r *Registry) ApplyLayout(slots []Slot) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(slots) > len(r.cards) {
		return fmt.Errorf("layout has %d slots for %d live cards", len(slots), len(r.cards))
	}
	seenCards := make(map[string]struct{}, len(slots))
	seenOrders := make(map[int]struct{}, len(slots))
	maxOrder := 0
	for _, slot := range slots {
		if _, ok := r.cards[slot.SurfaceID]; !ok {
			return fmt.Errorf("layout references unknown card %q", slot.SurfaceID)
		}
		if _, duplicate := seenCards[slot.SurfaceID]; duplicate {
			return fmt.Errorf("layout repeats card %q", slot.SurfaceID)
		}
		if slot.Order <= 0 {
			return fmt.Errorf("layout order for %q must be positive", slot.SurfaceID)
		}
		if _, duplicate := seenOrders[slot.Order]; duplicate {
			return fmt.Errorf("layout repeats order %d", slot.Order)
		}
		if slot.Span != 1 && slot.Span != 2 {
			return fmt.Errorf("layout span for %q must be 1 or 2", slot.SurfaceID)
		}
		seenCards[slot.SurfaceID] = struct{}{}
		seenOrders[slot.Order] = struct{}{}
		if slot.Order > maxOrder {
			maxOrder = slot.Order
		}
	}
	for _, slot := range slots {
		card := r.cards[slot.SurfaceID]
		card.Order = slot.Order
		card.Span = slot.Span
		r.cards[slot.SurfaceID] = card
	}

	// Cards that rendered after the curator read the registry keep their span
	// and follow the curated block, ordered among themselves as they were.
	uncurated := make([]Card, 0, len(r.cards)-len(seenCards))
	for _, card := range r.cards {
		if _, curated := seenCards[card.SurfaceID]; !curated {
			uncurated = append(uncurated, card)
		}
	}
	sort.Slice(uncurated, func(i, j int) bool {
		return uncurated[i].Order < uncurated[j].Order
	})
	for _, card := range uncurated {
		maxOrder++
		card.Order = maxOrder
		r.cards[card.SurfaceID] = card
	}

	if r.nextOrder <= maxOrder {
		r.nextOrder = maxOrder + 1
	}
	return nil
}

// Cards returns a stable append-order snapshot of every live card. Returned
// component trees and data models are deep clones and are safe for callers to
// retain or mutate.
func (r *Registry) Cards() []Card {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cards := make([]Card, 0, len(r.cards))
	for _, card := range r.cards {
		cards = append(cards, cloneCard(card))
	}
	sort.Slice(cards, func(i, j int) bool {
		return cards[i].Order < cards[j].Order
	})
	return cards
}

func cloneCard(card Card) Card {
	card.Components = cloneComponents(card.Components)
	card.DataModelSummary = cloneMap(card.DataModelSummary)
	return card
}

func cloneComponents(components []map[string]any) []map[string]any {
	if components == nil {
		return nil
	}
	data, err := json.Marshal(components)
	if err != nil {
		return []map[string]any{}
	}
	var clone []map[string]any
	if err := json.Unmarshal(data, &clone); err != nil {
		return []map[string]any{}
	}
	return clone
}

func cloneMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	data, err := json.Marshal(values)
	if err != nil {
		return map[string]any{}
	}
	var clone map[string]any
	if err := json.Unmarshal(data, &clone); err != nil {
		return map[string]any{}
	}
	return clone
}

// Kept local to avoid a formatting dependency for a tiny monotonically
// increasing identifier.
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}
