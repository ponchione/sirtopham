package agent

import (
	"log/slog"
	"reflect"
	"sync"
)

const defaultChannelSinkBuffer = 256

// EventSink is the transport-agnostic interface used by the agent loop to emit
// server-to-client events.
type EventSink interface {
	Emit(event Event)
	Close()
}

// MultiSink fans one emitted event out to multiple subscribers.
type MultiSink struct {
	mu     sync.RWMutex
	sinks  []EventSink
	closed bool
}

// NewMultiSink constructs an empty fan-out sink.
func NewMultiSink() *MultiSink {
	return &MultiSink{}
}

// Add registers another sink for future fan-out emission.
func (m *MultiSink) Add(sink EventSink) {
	if m == nil || isNilSink(sink) {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		sink.Close()
		return
	}
	for _, existing := range m.sinks {
		if sameSink(existing, sink) {
			return
		}
	}
	m.sinks = append(m.sinks, sink)
}

// Remove unregisters a sink from future fan-out emission.
func (m *MultiSink) Remove(sink EventSink) {
	if m == nil || isNilSink(sink) {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	filtered := m.sinks[:0]
	for _, existing := range m.sinks {
		if sameSink(existing, sink) {
			continue
		}
		filtered = append(filtered, existing)
	}
	m.sinks = filtered
}

// Emit forwards an event to every registered sink without blocking the caller.
func (m *MultiSink) Emit(event Event) {
	if m == nil || event == nil {
		return
	}

	m.mu.RLock()
	copied := append([]EventSink(nil), m.sinks...)
	m.mu.RUnlock()
	for _, sink := range copied {
		sink.Emit(event)
	}
}

// Close closes all currently registered sinks and prevents future additions.
func (m *MultiSink) Close() {
	if m == nil {
		return
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	copied := append([]EventSink(nil), m.sinks...)
	m.sinks = nil
	m.mu.Unlock()

	for _, sink := range copied {
		sink.Close()
	}
}

// ChannelSink buffers events on a channel for asynchronous consumption.
type ChannelSink struct {
	mu     sync.RWMutex
	ch     chan Event
	logger *slog.Logger
	closed bool
}

// NewChannelSink constructs a buffered channel-backed sink.
func NewChannelSink(bufferSize int) *ChannelSink {
	if bufferSize <= 0 {
		bufferSize = defaultChannelSinkBuffer
	}
	return &ChannelSink{
		ch:     make(chan Event, bufferSize),
		logger: slog.Default(),
	}
}

// Emit queues an event if buffer space is available. Under backpressure,
// droppable stream deltas are discarded before lifecycle/control events.
func (s *ChannelSink) Emit(event Event) {
	if s == nil || event == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.ch <- event:
		return
	default:
	}

	if !isDroppableAgentEvent(event) && s.evictDroppableEventLocked() {
		select {
		case s.ch <- event:
			return
		default:
		}
	}

	s.logger.Warn("dropping agent event", "event_type", event.EventType())
}

// Close closes the underlying event channel.
func (s *ChannelSink) Close() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.ch)
}

// Events exposes the read-only event stream.
func (s *ChannelSink) Events() <-chan Event {
	if s == nil {
		return nil
	}
	return s.ch
}

func (s *ChannelSink) evictDroppableEventLocked() bool {
	pending := len(s.ch)
	if pending == 0 {
		return false
	}

	kept := make([]Event, 0, pending)
	evicted := false
	for i := 0; i < pending; i++ {
		select {
		case queued := <-s.ch:
			if !evicted && isDroppableAgentEvent(queued) {
				evicted = true
				s.logger.Warn("dropping buffered agent event", "event_type", queued.EventType())
				continue
			}
			kept = append(kept, queued)
		default:
			i = pending
		}
	}
	for _, queued := range kept {
		s.ch <- queued
	}
	return evicted
}

func isDroppableAgentEvent(event Event) bool {
	if event == nil {
		return true
	}
	switch event.EventType() {
	case eventTypeToken, eventTypeThinkingDelta, eventTypeToolCallOutput, eventTypeContextDebug:
		return true
	default:
		return false
	}
}

func sameSink(a EventSink, b EventSink) bool {
	if isNilSink(a) || isNilSink(b) {
		return false
	}
	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)
	if va.Type() != vb.Type() {
		return false
	}
	if va.Type().Comparable() {
		return va.Interface() == vb.Interface()
	}
	return false
}

func isNilSink(sink EventSink) bool {
	if sink == nil {
		return true
	}
	value := reflect.ValueOf(sink)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
