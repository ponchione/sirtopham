package sse

import (
	"context"
	"strings"
	"testing"
)

func TestReaderParsesEventAndMultilineData(t *testing.T) {
	input := strings.NewReader(": keepalive\n\nevent: message_delta\ndata: {\"a\":1,\ndata: \"b\":2}\n\n")
	reader := NewReader(input, 0)

	event, ok, err := reader.Next(context.Background())
	if err != nil {
		t.Fatalf("Next returned error: %v", err)
	}
	if !ok {
		t.Fatal("Next ok = false, want true")
	}
	if event.Type != "message_delta" {
		t.Fatalf("Type = %q, want message_delta", event.Type)
	}
	if event.Data != "{\"a\":1,\n\"b\":2}" {
		t.Fatalf("Data = %q, want joined multiline payload", event.Data)
	}
}

func TestReaderFlushesTrailingEventAtEOF(t *testing.T) {
	reader := NewReader(strings.NewReader("event: done\ndata: [DONE]"), 0)

	event, ok, err := reader.Next(context.Background())
	if err != nil {
		t.Fatalf("Next returned error: %v", err)
	}
	if !ok {
		t.Fatal("Next ok = false, want true")
	}
	if event.Type != "done" || event.Data != "[DONE]" {
		t.Fatalf("event = %#v, want done/[DONE]", event)
	}

	if _, ok, err := reader.Next(context.Background()); err != nil || ok {
		t.Fatalf("second Next = ok %t err %v, want false nil", ok, err)
	}
}

func TestReaderReportsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader := NewReader(strings.NewReader("data: ignored\n\n"), 0)

	_, ok, err := reader.Next(ctx)
	if err == nil {
		t.Fatal("Next err = nil, want cancellation error")
	}
	if ok {
		t.Fatal("Next ok = true, want false")
	}
}
