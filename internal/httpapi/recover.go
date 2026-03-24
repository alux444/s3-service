package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/go-chi/chi/v5/middleware"
)

// middleware that catches panics and returns a JSON error response instead of crashing the server
func recoverJSON(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						"panic", fmt.Sprintf("%v", rec),
						"stack", string(debug.Stack()),
						"path", r.URL.Path,
						"method", r.Method,
						"request_id", middleware.GetReqID(r.Context()),
					)
					writeError(w, r, http.StatusInternalServerError, "internal_server_error", "An unexpected error occurred", nil)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
