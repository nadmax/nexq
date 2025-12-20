// Package httputil contains shared HTTP utilities for consistent response formatting across handlers.
package httputil

import (
	"encoding/json"
	"net/http"
)

func WriteJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
