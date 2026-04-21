package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

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
			requestID := chimiddleware.GetReqID(r.Context())
			tokenString, ok := bearerTokenFromHeader(r.Header.Get("Authorization"))
			if !ok {
				logger.Info("auth_rejected_missing_bearer_token",
					"method", r.Method,
					"path", r.URL.Path,
					"request_id", requestID,
				)
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
				logger.Info("auth_rejected_invalid_token",
					"method", r.Method,
					"path", r.URL.Path,
					"request_id", requestID,
					"reason", reason,
				)
				httpapi.WriteError(w, r, http.StatusUnauthorized, "auth_failed", "invalid authorization token", httpapi.AuthDetails{Reason: reason})
				return
			}

			logger.Info("auth_verified",
				"method", r.Method,
				"path", r.URL.Path,
				"request_id", requestID,
				"principal_type", claims.PrincipalType,
				"subject", claims.Subject,
				"project_id", claims.ProjectID,
				"app_id", claims.AppID,
			)
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
