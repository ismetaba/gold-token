package httputil

import (
	"encoding/json"
	"net/http"

	golderrors "github.com/ismetaba/gold-token/backend/pkg/errors"
)

// WriteJSON writes v as a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes the canonical error envelope:
//
//	{"error": {"code": "<code>", "message": "<message>"}}
//
// This is the single source of truth for the backend error wire format, so
// every service renders errors identically (and matches what the web client
// parses: error.code / error.message).
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

// WriteCodedError renders a *errors.Error using its embedded HTTP status, stable
// GOLD code, message, and any structured metadata. It lets handlers surface the
// canonical coded errors defined in pkg/errors without re-deriving the status.
func WriteCodedError(w http.ResponseWriter, err *golderrors.Error) {
	status := err.HTTP
	if status == 0 {
		status = http.StatusInternalServerError
	}
	body := map[string]any{
		"code":    string(err.Code),
		"message": err.Message,
	}
	if len(err.Meta) > 0 {
		body["meta"] = err.Meta
	}
	WriteJSON(w, status, map[string]any{"error": body})
}
