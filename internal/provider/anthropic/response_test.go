package anthropic

import (
	"io"
	"strings"
	"testing"
)

type countingReader struct {
	remaining int
	totalRead int
}

func (r *countingReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}

	n := len(p)
	if n > r.remaining {
		n = r.remaining
	}
	for i := 0; i < n; i++ {
		p[i] = 'x'
	}

	r.remaining -= n
	r.totalRead += n
	return n, nil
}

func TestReadResponseBodyLimitsBytesRead(t *testing.T) {
	reader := &countingReader{remaining: maxResponseSize + 1024}

	body, err := readResponseBody(reader)
	if err != nil {
		t.Fatalf("readResponseBody: %v", err)
	}

	if len(body) != maxResponseSize {
		t.Fatalf("expected %d bytes, got %d", maxResponseSize, len(body))
	}
	if reader.totalRead != maxResponseSize {
		t.Fatalf("expected to read %d bytes, read %d", maxResponseSize, reader.totalRead)
	}
}

func TestReadResponseBodyReturnsSmallBodyUnchanged(t *testing.T) {
	want := `{"type":"message"}`

	body, err := readResponseBody(strings.NewReader(want))
	if err != nil {
		t.Fatalf("readResponseBody: %v", err)
	}
	if string(body) != want {
		t.Fatalf("expected %q, got %q", want, string(body))
	}
}
