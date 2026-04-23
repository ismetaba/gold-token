// Package errors provides structured, coded errors for the GOLD backend.
//
// Error codes follow the pattern GOLD.<DOMAIN>.<NUMBER>, e.g. GOLD.ORDER.001.
// See docs/backend/error-codes.md for the canonical list.
package errors

import (
	stderrors "errors"
	"fmt"
)

// Code is a stable, machine-readable error identifier.
type Code string

const (
	// Ortak
	CodeInternal      Code = "GOLD.CORE.001"
	CodeNotFound      Code = "GOLD.CORE.002"
	CodeValidation    Code = "GOLD.CORE.003"
	CodeUnauthorized  Code = "GOLD.CORE.004"
	CodeForbidden     Code = "GOLD.CORE.005"
	CodeConflict      Code = "GOLD.CORE.006"
	CodeTooManyReqs   Code = "GOLD.CORE.007"
	CodeIdempotency   Code = "GOLD.CORE.008"

	// Mint/Burn
	CodeMintInvariant      Code = "GOLD.MINT.001" // totalSupply + amount > attestedGrams
	CodeMintStaleReserve   Code = "GOLD.MINT.002"
	CodeMintBarsNotAvail   Code = "GOLD.MINT.003"
	CodeMintAlreadyUsed    Code = "GOLD.MINT.004"
	CodeMintRejectedByComp Code = "GOLD.MINT.005"
	CodeMintApprovalsLow   Code = "GOLD.MINT.006"

	CodeBurnInsufficientBal Code = "GOLD.BURN.001"
	CodeBurnMinNotMet       Code = "GOLD.BURN.002"
)

// Error is the canonical error type for backend services.
type Error struct {
	Code      Code
	Message   string
	HTTP      int
	Retryable bool
	Meta      map[string]any
	Wrapped   error
}

func (e *Error) Error() string {
	if e.Wrapped != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Wrapped)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Wrapped }

// New creates a new Error with required fields.
func New(code Code, message string, httpStatus int) *Error {
	return &Error{Code: code, Message: message, HTTP: httpStatus}
}

// Wrap attaches an underlying error to an existing coded Error.
func (e *Error) Wrap(err error) *Error {
	cp := *e
	cp.Wrapped = err
	return &cp
}

// WithMeta attaches structured metadata (surfaced in API response).
func (e *Error) WithMeta(key string, val any) *Error {
	cp := *e
	if cp.Meta == nil {
		cp.Meta = make(map[string]any, 2)
	} else {
		// shallow-clone to avoid mutating shared base
		m := make(map[string]any, len(cp.Meta)+1)
		for k, v := range cp.Meta {
			m[k] = v
		}
		cp.Meta = m
	}
	cp.Meta[key] = val
	return &cp
}

// Retry marks the error as safe to retry idempotently.
func (e *Error) Retry() *Error {
	cp := *e
	cp.Retryable = true
	return &cp
}

// As is a thin wrapper around stdlib errors.As, keyed on *Error.
func As(err error) (*Error, bool) {
	var target *Error
	ok := stderrors.As(err, &target)
	return target, ok
}
