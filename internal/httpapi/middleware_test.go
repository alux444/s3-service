package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"s3-service/internal/auth"
)

type stubVerifier struct {
	claims auth.Claims
	err    error
	token  string
}

func (s *stubVerifier) Verify(tokenString string) (auth.Claims, error) {
	return s.VerifyWithContext(context.Background(), tokenString)
}

func (s *stubVerifier) VerifyWithContext(_ context.Context, tokenString string) (auth.Claims, error) {
	s.token = tokenString
	if s.err != nil {
		return auth.Claims{}, s.err
	}
	return s.claims, nil
}

type authErrorResponse struct {
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details *struct {
			Reason string `json:"reason"`
		} `json:"details"`
	} `json:"error"`
}

func decodeAuthError(t *testing.T, body io.Reader) authErrorResponse {
	t.Helper()

	var got authErrorResponse
	if err := json.NewDecoder(body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got.Error == nil {
		t.Fatal("expected error envelope")
	}
	return got
}

func TestJWTAuthMiddleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("missing authorization header", func(t *testing.T) {
		v := &stubVerifier{}
		h := JWTAuthMiddleware(logger, v)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("next handler should not be called")
		}))

		req := httptest.NewRequest(http.MethodGet, "/v1/auth-check", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
		got := decodeAuthError(t, rec.Body)
		if got.Error.Code != "auth_failed" {
			t.Fatalf("expected code auth_failed, got %s", got.Error.Code)
		}
		if got.Error.Details == nil || got.Error.Details.Reason != "missing" {
			t.Fatalf("expected reason missing, got %+v", got.Error.Details)
		}
	})

	t.Run("non bearer scheme", func(t *testing.T) {
		v := &stubVerifier{}
		h := JWTAuthMiddleware(logger, v)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("next handler should not be called")
		}))

		req := httptest.NewRequest(http.MethodGet, "/v1/auth-check", nil)
		req.Header.Set("Authorization", "Basic abc")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
		got := decodeAuthError(t, rec.Body)
		if got.Error.Details == nil || got.Error.Details.Reason != "missing" {
			t.Fatalf("expected reason missing, got %+v", got.Error.Details)
		}
	})

	t.Run("expired token maps reason", func(t *testing.T) {
		v := &stubVerifier{err: auth.ErrTokenExpired}
		h := JWTAuthMiddleware(logger, v)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("next handler should not be called")
		}))

		req := httptest.NewRequest(http.MethodGet, "/v1/auth-check", nil)
		req.Header.Set("Authorization", "Bearer expired-token")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
		got := decodeAuthError(t, rec.Body)
		if got.Error.Details == nil || got.Error.Details.Reason != "expired" {
			t.Fatalf("expected reason expired, got %+v", got.Error.Details)
		}
	})

	t.Run("invalid token maps reason", func(t *testing.T) {
		v := &stubVerifier{err: errors.New("bad token")}
		h := JWTAuthMiddleware(logger, v)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("next handler should not be called")
		}))

		req := httptest.NewRequest(http.MethodGet, "/v1/auth-check", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
		got := decodeAuthError(t, rec.Body)
		if got.Error.Details == nil || got.Error.Details.Reason != "invalid" {
			t.Fatalf("expected reason invalid, got %+v", got.Error.Details)
		}
	})

	t.Run("accepts case insensitive bearer and forwards claims", func(t *testing.T) {
		v := &stubVerifier{claims: auth.Claims{Subject: "user-1", AppID: "app-1"}}
		h := JWTAuthMiddleware(logger, v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				t.Fatal("claims not found in context")
			}
			if claims.Subject != "user-1" || claims.AppID != "app-1" {
				t.Fatalf("unexpected claims: %+v", claims)
			}
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/v1/auth-check", nil)
		req.Header.Set("Authorization", "bearer token-123")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if v.token != "token-123" {
			t.Fatalf("expected raw token token-123, got %s", v.token)
		}
	})
}
