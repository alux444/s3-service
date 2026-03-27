package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"s3-service/internal/httpapi"
)

func TestWriteJSON(t *testing.T) {
	t.Run("sets content-type header", func(t *testing.T) {
		w := httptest.NewRecorder()
		httpapi.WriteJSON(w, http.StatusOK, map[string]string{"key": "value"})

		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
	})

	t.Run("writes correct status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		httpapi.WriteJSON(w, http.StatusTeapot, nil)

		if w.Code != http.StatusTeapot {
			t.Errorf("expected status %d, got %d", http.StatusTeapot, w.Code)
		}
	})

	t.Run("encodes payload as json", func(t *testing.T) {
		w := httptest.NewRecorder()
		httpapi.WriteJSON(w, http.StatusOK, map[string]string{"hello": "world"})

		var got map[string]string
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if got["hello"] != "world" {
			t.Errorf("expected hello=world, got hello=%s", got["hello"])
		}
	})
}

func TestWriteOK(t *testing.T) {
	t.Run("returns 200 with data envelope", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		httpapi.WriteOK(w, r, map[string]string{"id": "123"})

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		var got struct {
			Data map[string]string `json:"data"`
			Err  any               `json:"error"`
		}
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if got.Data == nil {
			t.Fatal("expected data to be set, got nil")
		}
		if got.Data["id"] != "123" {
			t.Errorf("expected id=123, got id=%s", got.Data["id"])
		}
		if got.Err != nil {
			t.Error("expected error to be nil")
		}
	})

	t.Run("nil payload omits data field", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		httpapi.WriteOK(w, r, nil)

		var got struct {
			Data any `json:"data"`
		}
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if got.Data != nil {
			t.Errorf("expected data to be nil, got %v", got.Data)
		}
	})
}

func TestWriteCreated(t *testing.T) {
	t.Run("returns 201 with data envelope", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", nil)

		httpapi.WriteCreated(w, r, map[string]string{"id": "456"})

		if w.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		var got struct {
			Data map[string]string `json:"data"`
			Err  any               `json:"error"`
		}
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if got.Data == nil {
			t.Fatal("expected data to be set, got nil")
		}
		if got.Err != nil {
			t.Error("expected error to be nil")
		}
	})
}

func TestWriteNoContent(t *testing.T) {
	t.Run("returns 204 with no body", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodDelete, "/", nil)

		httpapi.WriteNoContent(w, r)

		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", w.Code)
		}
		if w.Body.Len() != 0 {
			t.Errorf("expected empty body, got %q", w.Body.String())
		}
	})
}
