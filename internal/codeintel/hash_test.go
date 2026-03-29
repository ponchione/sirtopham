package codeintel

import "testing"

func TestChunkID(t *testing.T) {
	t.Run("deterministic and expected format", func(t *testing.T) {
		got1 := ChunkID("internal/auth/middleware.go", ChunkTypeFunction, "AuthMiddleware", 15)
		got2 := ChunkID("internal/auth/middleware.go", ChunkTypeFunction, "AuthMiddleware", 15)

		if got1 != got2 {
			t.Fatalf("ChunkID not deterministic: %q != %q", got1, got2)
		}
		assertSHA256Hex(t, got1)
	})

	t.Run("known value", func(t *testing.T) {
		got := ChunkID("main.go", ChunkTypeFunction, "main", 1)
		want := "411fa97556c438d37bed36cad33112a3865f67223c04313ada02a2e10f3a524a"
		if got != want {
			t.Fatalf("ChunkID() = %q, want %q", got, want)
		}
	})

	t.Run("collision resistance", func(t *testing.T) {
		base := ChunkID("a.go", ChunkTypeFunction, "Foo", 10)
		cases := []struct {
			name string
			got  string
		}{
			{name: "different line", got: ChunkID("a.go", ChunkTypeFunction, "Foo", 20)},
			{name: "different file", got: ChunkID("b.go", ChunkTypeFunction, "Foo", 10)},
			{name: "different chunk type", got: ChunkID("a.go", ChunkTypeMethod, "Foo", 10)},
		}

		for _, tc := range cases {
			if tc.got == base {
				t.Fatalf("ChunkID collision for %s: %q", tc.name, tc.got)
			}
		}
	})

	t.Run("empty name still valid", func(t *testing.T) {
		got := ChunkID("a.go", ChunkTypeFallback, "", 1)
		assertSHA256Hex(t, got)
	})
}

func TestContentHash(t *testing.T) {
	t.Run("deterministic and expected format", func(t *testing.T) {
		got1 := ContentHash("func main() { fmt.Println(\"hello\") }")
		got2 := ContentHash("func main() { fmt.Println(\"hello\") }")

		if got1 != got2 {
			t.Fatalf("ContentHash not deterministic: %q != %q", got1, got2)
		}
		assertSHA256Hex(t, got1)
	})

	t.Run("known value", func(t *testing.T) {
		got := ContentHash("hello")
		want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
		if got != want {
			t.Fatalf("ContentHash() = %q, want %q", got, want)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		got := ContentHash("")
		want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		if got != want {
			t.Fatalf("ContentHash(\"\") = %q, want %q", got, want)
		}
	})

	t.Run("change detection", func(t *testing.T) {
		v1 := ContentHash("version 1")
		v2 := ContentHash("version 2")
		if v1 == v2 {
			t.Fatalf("expected different hashes for changed content, got %q", v1)
		}
	})
}

func assertSHA256Hex(t *testing.T, got string) {
	t.Helper()

	if len(got) != 64 {
		t.Fatalf("hash length = %d, want 64", len(got))
	}
	for _, r := range got {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			t.Fatalf("hash contains non-lowercase-hex rune %q in %q", r, got)
		}
	}
}
