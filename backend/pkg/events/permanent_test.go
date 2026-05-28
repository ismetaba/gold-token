package events

import (
	"errors"
	"fmt"
	"testing"
)

func TestPermanentNilStaysNil(t *testing.T) {
	if Permanent(nil) != nil {
		t.Fatal("Permanent(nil) should be nil")
	}
}

func TestIsPermanent(t *testing.T) {
	base := errors.New("bad payload")

	if IsPermanent(base) {
		t.Fatal("a plain error must not be permanent")
	}
	if !IsPermanent(Permanent(base)) {
		t.Fatal("a wrapped error must be permanent")
	}
	// Survives further wrapping.
	wrapped := fmt.Errorf("context: %w", Permanent(base))
	if !IsPermanent(wrapped) {
		t.Fatal("permanence must propagate through fmt.Errorf %w wrapping")
	}
	// Unwraps to the original error.
	if !errors.Is(wrapped, base) {
		t.Fatal("original error must remain unwrappable")
	}
}
