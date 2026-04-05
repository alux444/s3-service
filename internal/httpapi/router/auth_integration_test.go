package router_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"s3-service/internal/auth"
	"s3-service/internal/database"
	httpmiddleware "s3-service/internal/httpapi/middleware"
	"s3-service/internal/httpapi/router"
	"s3-service/internal/service"
)

type errorEnvelope struct {
	Error *errorBody `json:"error"`
}

type errorBody struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details *authDetails `json:"details"`
}

type authDetails struct {
	Reason string `json:"reason"`
}

type integrationAuthRepo struct{}

func (r *integrationAuthRepo) GetEffectiveAuthorizationPolicy(_ context.Context, lookup database.AuthorizationPolicyLookup) (database.EffectiveAuthorizationPolicy, error) {
	if lookup.ProjectID != "project-1" || lookup.PrincipalType == "" || lookup.PrincipalID == "" || lookup.BucketName != "bucket-a" {
		return database.EffectiveAuthorizationPolicy{}, database.ErrPolicyNotFound
	}

	if lookup.AppID != "app-allowed" {
		return database.EffectiveAuthorizationPolicy{}, database.ErrPolicyNotFound
	}

	return database.EffectiveAuthorizationPolicy{
		CanWrite:           true,
		ConnectionPrefixes: []string{"uploads/private/"},
		PrincipalPrefixes:  []string{"uploads/private/"},
	}, nil
}

func TestAuthIntegration_InvalidToken(t *testing.T) {
	_, jwksURL := newJWKS(t)
	verifier := newVerifier(t, jwksURL, "s3-service")

	r := router.NewRouter(
		testLogger(),
		httpmiddleware.JWTAuthMiddleware(testLogger(), verifier),
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth-check", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
	got := decodeError(t, rec)
	if got.Error == nil || got.Error.Code != "auth_failed" {
		t.Fatalf("expected auth_failed, got %+v", got.Error)
	}
	if got.Error.Details == nil || got.Error.Details.Reason != "invalid" {
		t.Fatalf("expected invalid reason, got %+v", got.Error.Details)
	}
}

func TestAuthIntegration_WrongAudience(t *testing.T) {
	privateKey, jwksURL := newJWKS(t)
	verifier := newVerifier(t, jwksURL, "s3-service")

	r := router.NewRouter(
		testLogger(),
		httpmiddleware.JWTAuthMiddleware(testLogger(), verifier),
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	token := signedToken(t, privateKey, map[string]any{
		"sub":            "user-1",
		"app_id":         "app-allowed",
		"project_id":     "project-1",
		"role":           string(auth.RoleProjectClient),
		"principal_type": string(auth.PrincipalTypeUser),
		"iss":            "https://issuer.test",
		"aud":            "another-audience",
		"exp":            time.Now().Add(5 * time.Minute).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth-check", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
	got := decodeError(t, rec)
	if got.Error == nil || got.Error.Code != "auth_failed" {
		t.Fatalf("expected auth_failed, got %+v", got.Error)
	}
	if got.Error.Details == nil || got.Error.Details.Reason != "invalid" {
		t.Fatalf("expected invalid reason, got %+v", got.Error.Details)
	}
}

func TestAuthIntegration_ForbiddenPrefixScope(t *testing.T) {
	privateKey, jwksURL := newJWKS(t)
	verifier := newVerifier(t, jwksURL, "s3-service")
	authzSvc := service.NewAuthorizationService(&integrationAuthRepo{})

	r := router.NewRouter(
		testLogger(),
		httpmiddleware.JWTAuthMiddleware(testLogger(), verifier),
		nil,
		authzSvc,
		nil,
		nil,
		nil,
	)

	token := signedToken(t, privateKey, map[string]any{
		"sub":            "user-1",
		"app_id":         "app-allowed",
		"project_id":     "project-1",
		"role":           string(auth.RoleProjectClient),
		"principal_type": string(auth.PrincipalTypeUser),
		"iss":            "https://issuer.test",
		"aud":            "s3-service",
		"exp":            time.Now().Add(5 * time.Minute).Unix(),
	})

	body := `{"bucket_name":"bucket-a","object_key":"uploads/public/a.jpg","content_type":"image/jpeg","content_b64":"cGF5bG9hZA=="}`
	req := httptest.NewRequest(http.MethodPost, "/v1/objects/upload", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
	got := decodeError(t, rec)
	if got.Error == nil || got.Error.Code != "forbidden" {
		t.Fatalf("expected forbidden code, got %+v", got.Error)
	}
	if got.Error.Details == nil || got.Error.Details.Reason != auth.DecisionReasonPrefixScope {
		t.Fatalf("expected prefix_scope reason, got %+v", got.Error.Details)
	}
}

func TestAuthIntegration_AppScopeDenial(t *testing.T) {
	privateKey, jwksURL := newJWKS(t)
	verifier := newVerifier(t, jwksURL, "s3-service")
	authzSvc := service.NewAuthorizationService(&integrationAuthRepo{})

	r := router.NewRouter(
		testLogger(),
		httpmiddleware.JWTAuthMiddleware(testLogger(), verifier),
		nil,
		authzSvc,
		nil,
		nil,
		nil,
	)

	token := signedToken(t, privateKey, map[string]any{
		"sub":            "user-1",
		"app_id":         "app-other",
		"project_id":     "project-1",
		"role":           string(auth.RoleProjectClient),
		"principal_type": string(auth.PrincipalTypeUser),
		"iss":            "https://issuer.test",
		"aud":            "s3-service",
		"exp":            time.Now().Add(5 * time.Minute).Unix(),
	})

	body := `{"bucket_name":"bucket-a","object_key":"uploads/private/a.jpg","content_type":"image/jpeg","content_b64":"cGF5bG9hZA=="}`
	req := httptest.NewRequest(http.MethodPost, "/v1/objects/upload", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
	got := decodeError(t, rec)
	if got.Error == nil || got.Error.Code != "forbidden" {
		t.Fatalf("expected forbidden code, got %+v", got.Error)
	}
	if got.Error.Details == nil || got.Error.Details.Reason != auth.DecisionReasonBucketScope {
		t.Fatalf("expected bucket_scope reason, got %+v", got.Error.Details)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) errorEnvelope {
	t.Helper()

	var got errorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	return got
}

func newVerifier(t *testing.T, jwksURL string, audience string) *auth.JWTVerifier {
	t.Helper()

	verifier, err := auth.NewJWTVerifier(auth.Config{
		Issuer:   "https://issuer.test",
		Audience: audience,
		JWKSURL:  jwksURL,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}
	t.Cleanup(verifier.Close)
	return verifier
}

func newJWKS(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key: %v", err)
	}

	n := base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes())
	jwksBody := map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"kid": "test-kid",
				"use": "sig",
				"alg": "RS256",
				"n":   n,
				"e":   e,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwksBody)
	}))
	t.Cleanup(server.Close)

	return privateKey, server.URL
}

func signedToken(t *testing.T, privateKey *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims(claims))
	tok.Header["kid"] = "test-kid"
	out, err := tok.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return out
}
