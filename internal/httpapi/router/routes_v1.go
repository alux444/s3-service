package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"s3-service/internal/httpapi/handlers"
)

func registerV1Routes(r chi.Router, authMW func(http.Handler) http.Handler, auditMW func(http.Handler) http.Handler, rateLimitMW func(http.Handler) http.Handler, bucketService handlers.BucketConnectionService, authorizationService handlers.AuthorizationService) {
	r.Route("/v1", func(v1 chi.Router) {
		if auditMW != nil {
			v1.Use(auditMW)
		}
		v1.Use(authMW)
		if rateLimitMW != nil {
			v1.Use(rateLimitMW)
		}
		v1.Get("/auth-check", handlers.AuthCheckHandler)

		if bucketService != nil {
			v1.Get("/bucket-connections", handlers.ListBucketConnectionsHandler(bucketService))
		}
		if authorizationService != nil {
			v1.Post("/objects/upload", handlers.UploadObjectHandler(authorizationService))
			v1.Delete("/objects", handlers.DeleteObjectHandler(authorizationService))
			v1.Post("/objects/presign-upload", handlers.PresignUploadObjectHandler(authorizationService))
			v1.Post("/objects/presign-download", handlers.PresignDownloadObjectHandler(authorizationService))
		}
	})
}
