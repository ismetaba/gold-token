package repo

import "testing"

func TestParseBigInt(t *testing.T) {
	n, err := parseBigInt("1000000000000000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.String() != "1000000000000000000" {
		t.Fatalf("got %s", n.String())
	}
}

func TestParseBigIntRejectsMalformed(t *testing.T) {
	// The critical property: malformed/empty input must error, never silently
	// yield a zero balance.
	for _, in := range []string{"", "abc", "1.5", "0x10", " 12 ", "NaN"} {
		if _, err := parseBigInt(in); err == nil {
			t.Errorf("parseBigInt(%q) returned nil error; expected failure", in)
		}
	}
}

func TestParseBigIntZero(t *testing.T) {
	n, err := parseBigInt("0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.Sign() != 0 {
		t.Fatalf("expected zero, got %s", n.String())
	}
}
