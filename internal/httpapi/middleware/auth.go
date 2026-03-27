package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"s3-service/internal/auth"
	"s3-service/internal/httpapi"
)

type claimsContextKey struct{}

type tokenVerifier interface {
	VerifyWithContext(ctx context.Context, tokenString string) (auth.Claims, error)
}

func JWTAuthMiddleware(logger *slog.Logger, verifier tokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString, ok := bearerTokenFromHeader(r.Header.Get("Authorization"))
			if !ok {
				httpapi.WriteError(w, r, http.StatusUnauthorized, "auth_failed", "missing bearer token", httpapi.AuthDetails{Reason: "missing"})
				return
			}

			claims, err := verifier.VerifyWithContext(r.Context(), tokenString)
			if err != nil {
				reason := "invalid"
				if errors.Is(err, auth.ErrTokenExpired) {
					reason = "expired"
				}

				logger.Warn("failed to verify JWT", "error", err, "reason", reason)
				httpapi.WriteError(w, r, http.StatusUnauthorized, "auth_failed", "invalid authorization token", httpapi.AuthDetails{Reason: reason})
				return
			}

			ctx := context.WithValue(r.Context(), claimsContextKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ClaimsFromContext(ctx context.Context) (auth.Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(auth.Claims)
	return claims, ok
}

func ContextWithClaims(ctx context.Context, claims auth.Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey{}, claims)
}

func bearerTokenFromHeader(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}

	return token, true
}
