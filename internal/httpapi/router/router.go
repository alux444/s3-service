package router

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"s3-service/internal/httpapi"
	"s3-service/internal/httpapi/handlers"
	httpmiddleware "s3-service/internal/httpapi/middleware"
)

func NewRouter(logger *slog.Logger, authMW func(http.Handler) http.Handler, bucketService handlers.BucketConnectionService, authorizationService handlers.AuthorizationService, objectUploadService handlers.ObjectUploadService, objectDeleteService handlers.ObjectDeleteService, objectPresignService handlers.ObjectPresignService, auditRecorder httpmiddleware.AuditEventRecorder) http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(httpmiddleware.RecoverJSON(logger))
	r.Use(httpmiddleware.RequestLogger(logger))

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		httpapi.WriteError(w, req, http.StatusNotFound, "not_found", "resource not found", httpapi.NotFoundDetails{
			Resource: "route",
			ID:       req.URL.Path,
		})
	})

	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		httpapi.WriteError(w, req, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
	})

	var auditMW func(http.Handler) http.Handler
	if auditRecorder != nil {
		auditMW = httpmiddleware.AuditEventsMiddleware(logger, auditRecorder)
	}
	rateLimitMW := httpmiddleware.NewIdentityIPRateLimitMiddleware(
		httpmiddleware.DefaultRateLimitPerWindow,
		httpmiddleware.DefaultRateLimitWindow,
	)

	r.Get("/health", handlers.HealthHandler)
	registerV1Routes(r, authMW, auditMW, rateLimitMW, bucketService, authorizationService, objectUploadService, objectDeleteService, objectPresignService)

	return r
}
