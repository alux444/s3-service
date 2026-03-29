package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseRole(t *testing.T) {
	t.Run("accepts supported roles", func(t *testing.T) {
		cases := []Role{RoleAdmin, RoleProjectClient, RoleReadOnlyClient}
		for _, tc := range cases {
			got, err := ParseRole(string(tc))
			if err != nil {
				t.Fatalf("expected no error for role %q, got %v", tc, err)
			}
			if got != tc {
				t.Fatalf("expected role %q, got %q", tc, got)
			}
		}
	})

	t.Run("rejects unsupported role", func(t *testing.T) {
		_, err := ParseRole("invalid_role")
		if err == nil {
			t.Fatal("expected error for invalid role")
		}
		if err != ErrInvalidRole {
			t.Fatalf("expected ErrInvalidRole, got %v", err)
		}
	})
}

func TestParsePrincipalType(t *testing.T) {
	t.Run("accepts supported principal types", func(t *testing.T) {
		cases := []PrincipalType{PrincipalTypeUser, PrincipalTypeService}
		for _, tc := range cases {
			got, err := ParsePrincipalType(string(tc))
			if err != nil {
				t.Fatalf("expected no error for principal type %q, got %v", tc, err)
			}
			if got != tc {
				t.Fatalf("expected principal type %q, got %q", tc, got)
			}
		}
	})

	t.Run("rejects unsupported principal type", func(t *testing.T) {
		_, err := ParsePrincipalType("machine")
		if err == nil {
			t.Fatal("expected error for invalid principal type")
		}
		if err != ErrInvalidPrincipalType {
			t.Fatalf("expected ErrInvalidPrincipalType, got %v", err)
		}
	})
}

func TestJWTVerifierVerifyWithContext_ServicePrincipal(t *testing.T) {
	privateKey, jwksURL := newTestSigningKeyAndJWKS(t)

	verifier, err := NewJWTVerifier(Config{
		Issuer:   "https://issuer.example.test",
		Audience: "s3-service",
		JWKSURL:  jwksURL,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}
	t.Cleanup(verifier.Close)

	token := newSignedToken(t, privateKey, map[string]any{
		"sub":            "svc-images",
		"app_id":         "app-1",
		"project_id":     "project-1",
		"role":           string(RoleProjectClient),
		"principal_type": string(PrincipalTypeService),
		"iss":            "https://issuer.example.test",
		"aud":            "s3-service",
		"exp":            time.Now().Add(5 * time.Minute).Unix(),
	})

	claims, err := verifier.VerifyWithContext(context.Background(), token)
	if err != nil {
		t.Fatalf("expected token to verify, got error: %v", err)
	}
	if claims.PrincipalType != PrincipalTypeService {
		t.Fatalf("expected principal type %q, got %q", PrincipalTypeService, claims.PrincipalType)
	}
	if claims.Subject != "svc-images" {
		t.Fatalf("expected subject svc-images, got %q", claims.Subject)
	}
}

func TestJWTVerifierVerifyWithContext_DefaultsPrincipalTypeToUser(t *testing.T) {
	privateKey, jwksURL := newTestSigningKeyAndJWKS(t)

	verifier, err := NewJWTVerifier(Config{
		Issuer:   "https://issuer.example.test",
		Audience: "s3-service",
		JWKSURL:  jwksURL,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}
	t.Cleanup(verifier.Close)

	token := newSignedToken(t, privateKey, map[string]any{
		"sub":        "user-1",
		"app_id":     "app-1",
		"project_id": "project-1",
		"role":       string(RoleProjectClient),
		"iss":        "https://issuer.example.test",
		"aud":        "s3-service",
		"exp":        time.Now().Add(5 * time.Minute).Unix(),
	})

	claims, err := verifier.VerifyWithContext(context.Background(), token)
	if err != nil {
		t.Fatalf("expected token to verify, got error: %v", err)
	}
	if claims.PrincipalType != PrincipalTypeUser {
		t.Fatalf("expected principal type %q, got %q", PrincipalTypeUser, claims.PrincipalType)
	}
}

func newTestSigningKeyAndJWKS(t *testing.T) (*rsa.PrivateKey, string) {
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
		if err := json.NewEncoder(w).Encode(jwksBody); err != nil {
			t.Fatalf("failed to write jwks response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return privateKey, server.URL
}

func newSignedToken(t *testing.T, privateKey *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims(claims))
	token.Header["kid"] = "test-kid"

	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	return tokenString
}
