package errors

import (
	stderrors "errors"
	"testing"
)

func TestErrorString(t *testing.T) {
	e := New(CodeValidation, "bad input", 422)
	if got := e.Error(); got != "GOLD.CORE.003: bad input" {
		t.Fatalf("got %q", got)
	}
	wrapped := e.Wrap(stderrors.New("root cause"))
	if got := wrapped.Error(); got != "GOLD.CORE.003: bad input: root cause" {
		t.Fatalf("got %q", got)
	}
}

func TestWrapDoesNotMutateBase(t *testing.T) {
	base := New(CodeInternal, "boom", 500)
	_ = base.Wrap(stderrors.New("cause"))
	if base.Wrapped != nil {
		t.Fatal("Wrap mutated the base error's Wrapped field")
	}
}

func TestWithMetaIsCopyOnWrite(t *testing.T) {
	base := New(CodeConflict, "conflict", 409)
	a := base.WithMeta("k", "v1")
	b := base.WithMeta("k", "v2")

	if base.Meta != nil {
		t.Fatal("WithMeta mutated the shared base error")
	}
	if a.Meta["k"] != "v1" || b.Meta["k"] != "v2" {
		t.Fatalf("metadata bled between copies: a=%v b=%v", a.Meta, b.Meta)
	}
}

func TestRetrySetsFlagOnCopy(t *testing.T) {
	base := New(CodeTooManyReqs, "slow down", 429)
	r := base.Retry()
	if base.Retryable {
		t.Fatal("Retry mutated the base error")
	}
	if !r.Retryable {
		t.Fatal("Retry did not set Retryable on the copy")
	}
}

func TestAsUnwrapsCodedError(t *testing.T) {
	inner := stderrors.New("inner")
	err := New(CodeNotFound, "missing", 404).Wrap(inner)
	got, ok := As(err)
	if !ok {
		t.Fatal("As failed to find *Error")
	}
	if got.Code != CodeNotFound {
		t.Fatalf("code=%s want %s", got.Code, CodeNotFound)
	}
	// stdlib errors.Is should reach the wrapped cause via Unwrap.
	if !stderrors.Is(err, inner) {
		t.Fatal("Unwrap chain broken")
	}
}
