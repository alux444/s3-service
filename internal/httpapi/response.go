package httpapi

import (
	"encoding/json"
	"net/http"
)

type envelope[T any] struct {
	Data  *T         `json:"data,omitempty"`
	Error *errorBody `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeOK(w http.ResponseWriter, _ *http.Request, payload any) {
	writeJSON(w, http.StatusOK, envelope[any]{Data: &payload})
}

func writeCreated(w http.ResponseWriter, _ *http.Request, payload any) {
	writeJSON(w, http.StatusCreated, envelope[any]{Data: &payload})
}

func writeNoContent(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
