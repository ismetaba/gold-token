package repo

import "testing"

func TestParseBigIntRejectsMalformed(t *testing.T) {
	for _, in := range []string{"", "abc", "0x10", "1e9"} {
		if _, err := parseBigInt(in); err == nil {
			t.Errorf("parseBigInt(%q) should error, not yield a silent zero", in)
		}
	}
	n, err := parseBigInt("31103476500000000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.String() != "31103476500000000000" {
		t.Fatalf("got %s", n.String())
	}
}
