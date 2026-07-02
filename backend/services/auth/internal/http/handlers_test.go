package http

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestDummyBcryptHashIsValid guards the user-enumeration timing defence: the
// dummy hash compared against on the not-found login path must be a real bcrypt
// hash at bcryptCost, so the comparison performs the same key-derivation work as
// a genuine wrong-password attempt. A malformed hash returns almost instantly and
// would re-open the timing oracle.
func TestDummyBcryptHashIsValid(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(dummyBcryptHash))
	if err != nil {
		t.Fatalf("dummyBcryptHash is not a valid bcrypt hash: %v", err)
	}
	if cost != bcryptCost {
		t.Fatalf("dummyBcryptHash cost = %d, want %d", cost, bcryptCost)
	}
	// A comparison must run (and fail) rather than error out on a malformed hash.
	if err := bcrypt.CompareHashAndPassword([]byte(dummyBcryptHash), []byte("some-password")); err == nil {
		t.Fatal("expected password mismatch against dummy hash")
	}
}
