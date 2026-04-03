package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"s3-service/internal/auth"
)

type rateLimitErrorEnvelope struct {
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details *struct {
			RetryAfter int `json:"retryAfter"`
			Limit      int `json:"limit"`
			Remaining  int `json:"remaining"`
		} `json:"details"`
	} `json:"error"`
}

func TestIdentityIPRateLimitMiddleware_BlocksAfterLimit(t *testing.T) {
	mw := NewIdentityIPRateLimitMiddleware(1, time.Minute)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	claims := auth.Claims{Subject: "user-1", PrincipalType: auth.PrincipalTypeUser}

	req1 := httptest.NewRequest(http.MethodGet, "/v1/auth-check", nil)
	req1.RemoteAddr = "203.0.113.10:43210"
	req1 = req1.WithContext(ContextWithClaims(req1.Context(), claims))
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first request status 200, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v1/auth-check", nil)
	req2.RemoteAddr = "203.0.113.10:43210"
	req2 = req2.WithContext(ContextWithClaims(req2.Context(), claims))
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request status 429, got %d", rec2.Code)
	}
	var got rateLimitErrorEnvelope
	if err := json.NewDecoder(rec2.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got.Error == nil || got.Error.Code != "rate_limited" {
		t.Fatalf("expected rate_limited response, got %+v", got.Error)
	}
	if got.Error.Details == nil || got.Error.Details.RetryAfter < 1 {
		t.Fatalf("expected retryAfter >= 1, got %+v", got.Error.Details)
	}
}

func TestIdentityIPRateLimitMiddleware_KeysByIdentityAndIP(t *testing.T) {
	mw := NewIdentityIPRateLimitMiddleware(1, time.Minute)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	user1 := auth.Claims{Subject: "user-1", PrincipalType: auth.PrincipalTypeUser}
	user2 := auth.Claims{Subject: "user-2", PrincipalType: auth.PrincipalTypeUser}

	reqA1 := httptest.NewRequest(http.MethodGet, "/v1/auth-check", nil)
	reqA1.RemoteAddr = "203.0.113.20:1111"
	reqA1 = reqA1.WithContext(ContextWithClaims(reqA1.Context(), user1))
	recA1 := httptest.NewRecorder()
	h.ServeHTTP(recA1, reqA1)
	if recA1.Code != http.StatusOK {
		t.Fatalf("expected user1 first request to pass, got %d", recA1.Code)
	}

	reqB1 := httptest.NewRequest(http.MethodGet, "/v1/auth-check", nil)
	reqB1.RemoteAddr = "203.0.113.20:2222"
	reqB1 = reqB1.WithContext(ContextWithClaims(reqB1.Context(), user2))
	recB1 := httptest.NewRecorder()
	h.ServeHTTP(recB1, reqB1)
	if recB1.Code != http.StatusOK {
		t.Fatalf("expected different identity same IP to pass, got %d", recB1.Code)
	}

	reqA2 := httptest.NewRequest(http.MethodGet, "/v1/auth-check", nil)
	reqA2.RemoteAddr = "203.0.113.21:3333"
	reqA2 = reqA2.WithContext(ContextWithClaims(reqA2.Context(), user1))
	recA2 := httptest.NewRecorder()
	h.ServeHTTP(recA2, reqA2)
	if recA2.Code != http.StatusOK {
		t.Fatalf("expected same identity different IP to pass, got %d", recA2.Code)
	}
}
