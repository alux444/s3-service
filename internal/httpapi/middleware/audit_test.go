package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"

	"s3-service/internal/auth"
	"s3-service/internal/database"
)

type stubAuditRecorder struct {
	events []database.AuditEventWrite
	err    error
}

func (s *stubAuditRecorder) RecordEvent(_ context.Context, event database.AuditEventWrite) error {
	s.events = append(s.events, event)
	return s.err
}

func TestAuditEventsMiddleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("records allowed user request", func(t *testing.T) {
		recorder := &stubAuditRecorder{}
		h := middleware.RequestID(AuditEventsMiddleware(logger, recorder)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})))

		req := httptest.NewRequest(http.MethodGet, "/v1/bucket-connections", nil)
		req = req.WithContext(ContextWithClaims(req.Context(), auth.Claims{
			Subject:       "user-1",
			ProjectID:     "project-1",
			AppID:         "app-1",
			PrincipalType: auth.PrincipalTypeUser,
		}))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if len(recorder.events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(recorder.events))
		}

		event := recorder.events[0]
		if event.Outcome != "allowed" || event.ActorType != "user" {
			t.Fatalf("unexpected event outcome/actor: %+v", event)
		}
		if event.ActorID != "user-1" || event.ProjectID != "project-1" || event.AppID != "app-1" {
			t.Fatalf("unexpected event scope fields: %+v", event)
		}
		if event.RequestID == "" {
			t.Fatal("expected requestID to be populated")
		}
		if event.Action != "bucket-connections" {
			t.Fatalf("expected action bucket-connections, got %q", event.Action)
		}
	})

	t.Run("records denied request when auth failed", func(t *testing.T) {
		recorder := &stubAuditRecorder{}
		h := middleware.RequestID(AuditEventsMiddleware(logger, recorder)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})))

		req := httptest.NewRequest(http.MethodGet, "/v1/auth-check", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
		if len(recorder.events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(recorder.events))
		}

		event := recorder.events[0]
		if event.Outcome != "denied" || event.ActorType != "system" {
			t.Fatalf("unexpected event outcome/actor: %+v", event)
		}
		if event.HTTPStatus != http.StatusUnauthorized {
			t.Fatalf("expected status 401 in event, got %d", event.HTTPStatus)
		}
		if event.Action != "auth-check" {
			t.Fatalf("expected action auth-check, got %q", event.Action)
		}
	})

	t.Run("records object route action name", func(t *testing.T) {
		recorder := &stubAuditRecorder{}
		h := middleware.RequestID(AuditEventsMiddleware(logger, recorder)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotImplemented)
		})))

		req := httptest.NewRequest(http.MethodPost, "/v1/objects/upload", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("expected status 501, got %d", rec.Code)
		}
		if len(recorder.events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(recorder.events))
		}

		event := recorder.events[0]
		if event.Action != "objects.upload" {
			t.Fatalf("expected action objects.upload, got %q", event.Action)
		}
		if event.Outcome != "error" {
			t.Fatalf("expected error outcome for 501, got %q", event.Outcome)
		}
	})
}
