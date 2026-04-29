package sse

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

const DefaultMaxEventBytes = 16 * 1024 * 1024

type Event struct {
	Type string
	Data string
}

type Reader struct {
	scanner   *bufio.Scanner
	eventType string
	dataLines []string
}

func NewReader(r io.Reader, maxTokenBytes int) *Reader {
	if maxTokenBytes <= 0 {
		maxTokenBytes = DefaultMaxEventBytes
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTokenBytes)
	return &Reader{scanner: scanner}
}

func (r *Reader) Next(ctx context.Context) (Event, bool, error) {
	for r.scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return Event{}, false, err
		}
		line := r.scanner.Text()
		if strings.TrimSpace(line) == "" {
			if ev, ok := r.flush(); ok {
				return ev, true, nil
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			r.eventType = value
		case "data":
			r.dataLines = append(r.dataLines, value)
		}
	}
	if err := r.scanner.Err(); err != nil {
		return Event{}, false, fmt.Errorf("read SSE stream: %w", err)
	}
	if ev, ok := r.flush(); ok {
		return ev, true, nil
	}
	return Event{}, false, nil
}

func (r *Reader) flush() (Event, bool) {
	if len(r.dataLines) == 0 {
		r.eventType = ""
		return Event{}, false
	}
	ev := Event{
		Type: r.eventType,
		Data: strings.Join(r.dataLines, "\n"),
	}
	r.eventType = ""
	r.dataLines = r.dataLines[:0]
	return ev, true
}
