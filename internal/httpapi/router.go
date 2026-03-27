package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(logger *slog.Logger, authMW func(http.Handler) http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(recoverJSON(logger))
	r.Use(requestLogger(logger))

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		writeError(w, req, http.StatusNotFound, "not_found", "resource not found", NotFoundDetails{
			Resource: "route",
			ID:       req.URL.Path,
		})
	})

	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		writeError(w, req, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
	})

	r.Get("/health", healthHandler)

	r.Route("/v1", func(v1 chi.Router) {
		v1.Use(authMW)
		v1.Get("/auth-check", func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				writeError(w, r, http.StatusUnauthorized, "auth_failed", "authentication required", AuthDetails{Reason: "missing"})
				return
			}

			writeOK(w, r, map[string]any{
				"sub":        claims.Subject,
				"app_id":     claims.AppID,
				"project_id": claims.ProjectID,
				"role":       claims.Role,
			})
		})
	})

	return r
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			logger.Info("http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}
