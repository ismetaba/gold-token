package repo

import "testing"

func TestParseBigIntRejectsMalformed(t *testing.T) {
	for _, in := range []string{"", "abc", "1.5", " 1 "} {
		if _, err := parseBigInt(in); err == nil {
			t.Errorf("parseBigInt(%q) should error, not yield a silent zero", in)
		}
	}
	n, err := parseBigInt("4200")
	if err != nil || n.Int64() != 4200 {
		t.Fatalf("parseBigInt(4200)=(%v,%v)", n, err)
	}
}
