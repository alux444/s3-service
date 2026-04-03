package httpapi

import (
	"encoding/json"
	"net/http"
)

type envelope[T any] struct {
	Data  *T         `json:"data,omitempty"`
	Error *errorBody `json:"error,omitempty"`
}

func WriteJSON[T any](w http.ResponseWriter, status int, payload T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func WriteOK[T any](w http.ResponseWriter, _ *http.Request, payload T) {
	WriteJSON(w, http.StatusOK, envelope[T]{Data: &payload})
}

func WriteCreated[T any](w http.ResponseWriter, _ *http.Request, payload T) {
	WriteJSON(w, http.StatusCreated, envelope[T]{Data: &payload})
}

func WriteNoContent(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
