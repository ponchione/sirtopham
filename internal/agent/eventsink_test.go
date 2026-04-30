package agent

import (
	"testing"
	"time"
)

func TestChannelSinkEmitCloseAndDropAreNonBlocking(t *testing.T) {
	sink := NewChannelSink(1)
	first := TokenEvent{Token: "first", Time: time.Unix(1700000300, 0).UTC()}
	second := TokenEvent{Token: "second", Time: time.Unix(1700000301, 0).UTC()}

	sink.Emit(first)
	done := make(chan struct{})
	go func() {
		sink.Emit(second)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Emit blocked with a full channel")
	}

	event := readEvent(t, sink.Events())
	token, ok := event.(TokenEvent)
	if !ok {
		t.Fatalf("event type = %T, want TokenEvent", event)
	}
	if token.Token != "first" {
		t.Fatalf("Token = %q, want first", token.Token)
	}

	select {
	case extra := <-sink.Events():
		t.Fatalf("unexpected extra event after full-buffer drop: %#v", extra)
	default:
	}

	sink.Close()
	if _, ok := <-sink.Events(); ok {
		t.Fatal("Events channel still open after Close")
	}
}

func TestChannelSinkEvictsDroppableEventForControlEvent(t *testing.T) {
	sink := NewChannelSink(1)
	sink.Emit(TokenEvent{Token: "streaming", Time: time.Unix(1700000300, 0).UTC()})

	sink.Emit(TurnCompleteEvent{TurnNumber: 1, IterationCount: 1, Time: time.Unix(1700000301, 0).UTC()})

	event := readEvent(t, sink.Events())
	complete, ok := event.(TurnCompleteEvent)
	if !ok {
		t.Fatalf("event type = %T, want TurnCompleteEvent", event)
	}
	if complete.TurnNumber != 1 {
		t.Fatalf("TurnNumber = %d, want 1", complete.TurnNumber)
	}
	select {
	case extra := <-sink.Events():
		t.Fatalf("unexpected extra event after control-event eviction: %#v", extra)
	default:
	}
}

func TestChannelSinkKeepsQueuedControlEventOverLaterDroppableEvent(t *testing.T) {
	sink := NewChannelSink(1)
	sink.Emit(ErrorEvent{ErrorCode: "terminal", Message: "failed", Time: time.Unix(1700000300, 0).UTC()})

	done := make(chan struct{})
	go func() {
		sink.Emit(TokenEvent{Token: "late", Time: time.Unix(1700000301, 0).UTC()})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Emit blocked with a full control-event channel")
	}

	event := readEvent(t, sink.Events())
	errEvent, ok := event.(ErrorEvent)
	if !ok {
		t.Fatalf("event type = %T, want ErrorEvent", event)
	}
	if errEvent.ErrorCode != "terminal" {
		t.Fatalf("ErrorCode = %q, want terminal", errEvent.ErrorCode)
	}
	select {
	case extra := <-sink.Events():
		t.Fatalf("unexpected extra event after droppable-event drop: %#v", extra)
	default:
	}
}

func TestMultiSinkFansOutAndRemove(t *testing.T) {
	multi := NewMultiSink()
	a := NewChannelSink(2)
	b := NewChannelSink(2)
	multi.Add(a)
	multi.Add(b)

	multi.Emit(StatusEvent{State: StateAssemblingContext, Time: time.Unix(1700000400, 0).UTC()})
	if got := readEvent(t, a.Events()).EventType(); got != "status" {
		t.Fatalf("sink a EventType() = %q, want status", got)
	}
	if got := readEvent(t, b.Events()).EventType(); got != "status" {
		t.Fatalf("sink b EventType() = %q, want status", got)
	}

	multi.Remove(a)
	multi.Emit(TokenEvent{Token: "after-remove", Time: time.Unix(1700000401, 0).UTC()})
	if got := readEvent(t, b.Events()).EventType(); got != "token" {
		t.Fatalf("sink b EventType() after remove = %q, want token", got)
	}
	select {
	case extra := <-a.Events():
		t.Fatalf("removed sink unexpectedly received event: %#v", extra)
	default:
	}
}

func readEvent(t *testing.T, ch <-chan Event) Event {
	t.Helper()
	select {
	case event := <-ch:
		return event
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for event")
		return nil
	}
}
