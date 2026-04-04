package langutil

import "testing"

func TestFromExtension(t *testing.T) {
	tests := []struct {
		ext  string
		want string
		ok   bool
	}{
		{".go", "go", true},
		{".tsx", "tsx", true},
		{".yaml", "yaml", true},
		{".unknown", "", false},
	}
	for _, tt := range tests {
		got, ok := FromExtension(tt.ext)
		if got != tt.want || ok != tt.ok {
			t.Fatalf("FromExtension(%q) = (%q, %v), want (%q, %v)", tt.ext, got, ok, tt.want, tt.ok)
		}
	}
}

func TestFromExtensionOr(t *testing.T) {
	if got := FromExtensionOr(".go", "text"); got != "go" {
		t.Fatalf("FromExtensionOr(.go) = %q, want go", got)
	}
	if got := FromExtensionOr(".unknown", "text"); got != "text" {
		t.Fatalf("FromExtensionOr(.unknown) = %q, want text", got)
	}
}
