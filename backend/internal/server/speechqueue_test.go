package server

import (
	"errors"
	"sync"
	"testing"
)

type speechRecorder struct {
	mu   sync.Mutex
	sent []string
	err  error
}

func (r *speechRecorder) send(text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	r.sent = append(r.sent, text)
	return nil
}

func (r *speechRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.sent...)
}

func TestSpeechQueueSendsImmediatelyWhenIdle(t *testing.T) {
	recorder := &speechRecorder{}
	queue := newSpeechQueue(recorder.send, nil)

	queue.Enqueue("first finding")

	if sent := recorder.snapshot(); len(sent) != 1 || sent[0] != "first finding" {
		t.Fatalf("idle queue did not speak immediately: %#v", sent)
	}
}

func TestSpeechQueueHoldsAndCoalescesWhileSpeaking(t *testing.T) {
	recorder := &speechRecorder{}
	queue := newSpeechQueue(recorder.send, nil)

	queue.SpeechStarted()
	queue.Enqueue("finding one")
	queue.Enqueue("finding two")

	if sent := recorder.snapshot(); len(sent) != 0 {
		t.Fatalf("findings interrupted speech in progress: %#v", sent)
	}

	queue.TurnComplete()

	sent := recorder.snapshot()
	if len(sent) != 1 {
		t.Fatalf("held findings were not coalesced into one turn: %#v", sent)
	}
	if sent[0] != "finding one\nfinding two" {
		t.Fatalf("unexpected coalesced turn: %q", sent[0])
	}
}

func TestSpeechQueueDropsHeldFindingsOnBargeIn(t *testing.T) {
	recorder := &speechRecorder{}
	queue := newSpeechQueue(recorder.send, nil)

	queue.SpeechStarted()
	queue.Enqueue("stale finding")
	queue.Interrupted()

	if sent := recorder.snapshot(); len(sent) != 0 {
		t.Fatalf("barge-in did not drop held findings: %#v", sent)
	}

	// The user's turn ended the previous speech, so the queue is idle again.
	queue.Enqueue("fresh finding")
	if sent := recorder.snapshot(); len(sent) != 1 || sent[0] != "fresh finding" {
		t.Fatalf("queue did not resume after barge-in: %#v", sent)
	}
}

func TestSpeechQueueSecondFindingWaitsForTheFirst(t *testing.T) {
	recorder := &speechRecorder{}
	queue := newSpeechQueue(recorder.send, nil)

	// Sending marks the queue busy even before any audio comes back, so a
	// finding arriving right behind it cannot cut off the reply it triggered.
	queue.Enqueue("first finding")
	queue.Enqueue("second finding")

	if sent := recorder.snapshot(); len(sent) != 1 {
		t.Fatalf("second finding did not wait: %#v", sent)
	}

	queue.TurnComplete()
	if sent := recorder.snapshot(); len(sent) != 2 || sent[1] != "second finding" {
		t.Fatalf("second finding was not released: %#v", sent)
	}
}

func TestSpeechQueueReportsSendFailureAndStaysUsable(t *testing.T) {
	recorder := &speechRecorder{err: errors.New("session gone")}
	var reported error
	queue := newSpeechQueue(recorder.send, func(err error) { reported = err })

	queue.Enqueue("doomed finding")
	if reported == nil {
		t.Fatal("send failure was not reported")
	}

	// A failed send must not leave the queue wedged as if speech were playing.
	recorder.mu.Lock()
	recorder.err = nil
	recorder.mu.Unlock()
	queue.Enqueue("later finding")
	if sent := recorder.snapshot(); len(sent) != 1 || sent[0] != "later finding" {
		t.Fatalf("queue wedged after a failed send: %#v", sent)
	}
}
