package server

import (
	"strings"
	"sync"
	"time"
)

// maxSpeechHold provides a safety timeout for held findings if TurnComplete
// is delayed or lost in transit.
const maxSpeechHold = 30 * time.Second

// speechQueue serializes agent findings into the live session so a result that
// lands mid-sentence no longer cuts off the speech already playing. Findings
// that arrive while the model is speaking are held and coalesced into a single
// follow-up turn. A user barge-in drops whatever is still held: the user's turn
// wins, and stale findings are not read out after the conversation moved on.
//
// Because nothing is ever sent while the model is speaking, any Interrupted
// event the session reports is necessarily the user talking over it.
type speechQueue struct {
	send    func(string) error
	onError func(error)

	mu       sync.Mutex
	pending  []string
	speaking bool
	timer    *time.Timer
}

func newSpeechQueue(send func(string) error, onError func(error)) *speechQueue {
	return &speechQueue{send: send, onError: onError}
}

// Enqueue speaks text now if the model is idle, or holds it until the current
// turn finishes.
func (q *speechQueue) Enqueue(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	q.mu.Lock()
	if q.speaking {
		q.pending = append(q.pending, text)
		q.mu.Unlock()
		return
	}
	q.mu.Unlock()
	q.dispatch([]string{text})
}

// SpeechStarted records that model output is in flight, so findings arriving
// from here until TurnComplete are held rather than sent.
func (q *speechQueue) SpeechStarted() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.speaking {
		return
	}
	q.speaking = true
	q.armLocked()
}

// TurnComplete releases anything held during the finished turn.
func (q *speechQueue) TurnComplete() {
	q.mu.Lock()
	q.speaking = false
	q.disarmLocked()
	held := q.takePendingLocked()
	q.mu.Unlock()
	q.dispatch(held)
}

// Interrupted drops held findings: the user barged in, so their turn wins.
func (q *speechQueue) Interrupted() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.speaking = false
	q.disarmLocked()
	q.pending = nil
}

// Reset clears all state when the live session ends.
func (q *speechQueue) Reset() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.speaking = false
	q.disarmLocked()
	q.pending = nil
}

// dispatch coalesces held findings into one turn so a burst of agent results
// becomes a single spoken response instead of a backlog.
func (q *speechQueue) dispatch(items []string) {
	if len(items) == 0 {
		return
	}
	q.mu.Lock()
	q.speaking = true
	q.armLocked()
	q.mu.Unlock()

	if err := q.send(strings.Join(items, "\n")); err != nil {
		q.mu.Lock()
		q.speaking = false
		q.disarmLocked()
		q.mu.Unlock()
		if q.onError != nil {
			q.onError(err)
		}
	}
}

func (q *speechQueue) takePendingLocked() []string {
	held := q.pending
	q.pending = nil
	return held
}

func (q *speechQueue) armLocked() {
	q.disarmLocked()
	q.timer = time.AfterFunc(maxSpeechHold, q.releaseStalled)
}

func (q *speechQueue) disarmLocked() {
	if q.timer != nil {
		q.timer.Stop()
		q.timer = nil
	}
}

// releaseStalled flushes held findings when a turn never reported completion.
func (q *speechQueue) releaseStalled() {
	q.mu.Lock()
	q.speaking = false
	q.timer = nil
	held := q.takePendingLocked()
	q.mu.Unlock()
	q.dispatch(held)
}
