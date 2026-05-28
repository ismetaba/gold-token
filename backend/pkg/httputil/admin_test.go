package httputil

import "testing"

func TestValidAdminSecret(t *testing.T) {
	cases := []struct {
		name       string
		configured string
		supplied   string
		want       bool
	}{
		{"match", "s3cret", "s3cret", true},
		{"mismatch", "s3cret", "wrong", false},
		{"empty supplied", "s3cret", "", false},
		{"empty configured (fail-safe)", "", "anything", false},
		{"both empty (fail-safe)", "", "", false},
		{"case sensitive", "Secret", "secret", false},
		{"length differs", "s3cret", "s3cretx", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidAdminSecret(tc.configured, tc.supplied); got != tc.want {
				t.Fatalf("ValidAdminSecret(%q,%q)=%v want %v", tc.configured, tc.supplied, got, tc.want)
			}
		})
	}
}
