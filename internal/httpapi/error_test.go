package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"s3-service/internal/httpapi"
	httpmiddleware "s3-service/internal/httpapi/middleware"
	"s3-service/internal/httpapi/router"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// capturingHandler implements slog.Handler and stores all records for inspection.
type capturingHandler struct {
	buf *bytes.Buffer
}

func (h *capturingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	r.Attrs(func(a slog.Attr) bool {
		h.buf.WriteString(a.Key + "=" + a.Value.String() + " ")
		return true
	})
	return nil
}
func (h *capturingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(_ string) slog.Handler      { return h }

type errorEnvelopeTestResponse struct {
	Error *struct {
		Code      string          `json:"code"`
		Message   string          `json:"message"`
		RequestId string          `json:"requestId"`
		Details   json.RawMessage `json:"details"`
	} `json:"error"`
	Data *json.RawMessage `json:"data"`
}

func decodeErrorEnvelope(t *testing.T, body io.Reader) errorEnvelopeTestResponse {
	t.Helper()

	var got errorEnvelopeTestResponse
	if err := json.NewDecoder(body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if got.Error == nil {
		t.Fatal("expected error envelope to be present")
	}

	return got
}

func assertDetailsAs[T any](t *testing.T, raw json.RawMessage, want T) {
	t.Helper()

	var got T
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("failed to unmarshal details: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unexpected details: got %+v want %+v", got, want)
	}
}

func TestWriteErrorEnvelope(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		code          string
		message       string
		details       httpapi.ErrorDetails
		expectDetails bool
	}{
		{
			name:          "without_details",
			status:        http.StatusBadRequest,
			code:          "bad_request",
			message:       "invalid request",
			details:       nil,
			expectDetails: false,
		},
		{
			name:          "with_validation_details",
			status:        http.StatusBadRequest,
			code:          "validation_failed",
			message:       "validation failed",
			details:       httpapi.ValidationDetails{Field: "bucket", Reason: "required"},
			expectDetails: true,
		},
		{
			name:          "with_not_found_details",
			status:        http.StatusNotFound,
			code:          "not_found",
			message:       "resource not found",
			details:       httpapi.NotFoundDetails{Resource: "image", ID: "img_123"},
			expectDetails: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				httpapi.WriteError(w, r, tc.status, tc.code, tc.message, tc.details)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Errorf("expected status %d, got %d", tc.status, rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", ct)
			}

			got := decodeErrorEnvelope(t, rec.Body)

			if got.Error.Code != tc.code {
				t.Errorf("expected code %s, got %s", tc.code, got.Error.Code)
			}
			if got.Error.Message != tc.message {
				t.Errorf("expected message %s, got %s", tc.message, got.Error.Message)
			}
			if got.Error.RequestId == "" {
				t.Error("expected requestId to be populated")
			}

			if tc.expectDetails {
				if len(got.Error.Details) == 0 {
					t.Fatal("expected details to be present")
				}
				switch details := tc.details.(type) {
				case httpapi.ValidationDetails:
					assertDetailsAs(t, got.Error.Details, details)
				case httpapi.NotFoundDetails:
					assertDetailsAs(t, got.Error.Details, details)
				default:
					t.Fatalf("unsupported details type: %T", details)
				}
			} else if len(got.Error.Details) != 0 && string(got.Error.Details) != "null" {
				t.Errorf("expected details to be omitted, got %s", string(got.Error.Details))
			}
		})
	}
}

func TestRecoverJSONPanicEnvelope(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(&capturingHandler{buf: buf})

	h := middleware.RequestID(httpmiddleware.RecoverJSON(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}

	got := decodeErrorEnvelope(t, rec.Body)
	if got.Error.Code != "internal_server_error" {
		t.Errorf("expected code internal_server_error, got %s", got.Error.Code)
	}
	if got.Error.RequestId == "" {
		t.Error("expected requestId to be populated")
	}

	logged := buf.String()
	if !strings.Contains(logged, "stack") {
		t.Error("expected stack trace to be logged")
	}
	if !strings.Contains(logged, "boom") {
		t.Error("expected panic value to be logged")
	}
}

func TestRouterErrorEnvelopes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := router.NewRouter(logger, func(next http.Handler) http.Handler { return next }, nil, nil, nil, nil, nil, nil)

	t.Run("not_found_includes_typed_details", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/missing", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", rec.Code)
		}

		got := decodeErrorEnvelope(t, rec.Body)
		if got.Error.Code != "not_found" {
			t.Errorf("expected code not_found, got %s", got.Error.Code)
		}
		assertDetailsAs(t, got.Error.Details, httpapi.NotFoundDetails{Resource: "route", ID: "/missing"})
	})

	t.Run("method_not_allowed_omits_details", func(t *testing.T) {
		r2 := chi.NewRouter()
		r2.Use(middleware.RequestID)
		r2.Use(httpmiddleware.RecoverJSON(logger))
		r2.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
			httpapi.WriteError(w, req, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
		})
		r2.Get("/only-get", func(w http.ResponseWriter, r *http.Request) {
			httpapi.WriteOK(w, r, map[string]string{"ok": "true"})
		})

		req := httptest.NewRequest(http.MethodPost, "/only-get", nil)
		rec := httptest.NewRecorder()
		r2.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", rec.Code)
		}

		got := decodeErrorEnvelope(t, rec.Body)
		if got.Error.Code != "method_not_allowed" {
			t.Errorf("expected code method_not_allowed, got %s", got.Error.Code)
		}
		if len(got.Error.Details) != 0 && string(got.Error.Details) != "null" {
			t.Errorf("expected no details, got %s", string(got.Error.Details))
		}
	})
}

func TestWriteError_WithDomainDetails(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		code    string
		message string
		details httpapi.ErrorDetails
	}{
		{
			name:    "auth_details",
			status:  http.StatusUnauthorized,
			code:    "auth_failed",
			message: "authentication failed",
			details: httpapi.AuthDetails{Reason: "expired"},
		},
		{
			name:    "rate_limit_details",
			status:  http.StatusTooManyRequests,
			code:    "throttle",
			message: "rate limit exceeded",
			details: httpapi.RateLimitDetails{RetryAfter: 30, Limit: 100, Remaining: 0},
		},
		{
			name:    "conflict_details",
			status:  http.StatusConflict,
			code:    "conflict",
			message: "resource already exists",
			details: httpapi.ConflictDetails{Resource: "bucket_connection", Field: "bucket_name", Value: "my-bucket"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				httpapi.WriteError(w, r, tc.status, tc.code, tc.message, tc.details)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Errorf("expected status %d, got %d", tc.status, rec.Code)
			}

			got := decodeErrorEnvelope(t, rec.Body)
			if got.Error.Code != tc.code {
				t.Errorf("expected code %s, got %s", tc.code, got.Error.Code)
			}
			if got.Error.Message != tc.message {
				t.Errorf("expected message %s, got %s", tc.message, got.Error.Message)
			}
			if got.Error.RequestId == "" {
				t.Error("expected requestId to be populated")
			}
			switch details := tc.details.(type) {
			case httpapi.AuthDetails:
				assertDetailsAs(t, got.Error.Details, details)
			case httpapi.RateLimitDetails:
				assertDetailsAs(t, got.Error.Details, details)
			case httpapi.ConflictDetails:
				assertDetailsAs(t, got.Error.Details, details)
			default:
				t.Fatalf("unsupported details type: %T", details)
			}
		})
	}
}
