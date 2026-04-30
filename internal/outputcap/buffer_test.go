package outputcap

import "testing"

func TestBufferCapsCapturedBytes(t *testing.T) {
	buf := NewBuffer(4)
	n, err := buf.Write([]byte("abcdef"))
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if n != 6 {
		t.Fatalf("Write count = %d, want 6", n)
	}
	if got := buf.String(); got != "abcd" {
		t.Fatalf("String = %q, want abcd", got)
	}
	if got := buf.TruncatedBytes(); got != 2 {
		t.Fatalf("TruncatedBytes = %d, want 2", got)
	}
}
