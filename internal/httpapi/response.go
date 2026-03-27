package httpapi

import (
	"encoding/json"
	"net/http"
)

type envelope[T any] struct {
	Data  *T         `json:"data,omitempty"`
	Error *errorBody `json:"error,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func WriteOK(w http.ResponseWriter, _ *http.Request, payload any) {
	WriteJSON(w, http.StatusOK, envelope[any]{Data: &payload})
}

func WriteCreated(w http.ResponseWriter, _ *http.Request, payload any) {
	WriteJSON(w, http.StatusCreated, envelope[any]{Data: &payload})
}

func WriteNoContent(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
