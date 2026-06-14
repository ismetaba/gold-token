package secrets

import "testing"

func TestRandomHexLengthAndEncoding(t *testing.T) {
	s, err := RandomHex(32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s) != 64 {
		t.Fatalf("len=%d want 64 hex chars for 32 bytes", len(s))
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("non-hex character %q in output", c)
		}
	}
}

func TestRandomHexUnique(t *testing.T) {
	a, _ := RandomHex(16)
	b, _ := RandomHex(16)
	if a == b {
		t.Fatal("two RandomHex calls returned identical values")
	}
}
