package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"s3-service/internal/httpapi/handlers"
)

func registerV1Routes(r chi.Router, authMW func(http.Handler) http.Handler, bucketService handlers.BucketConnectionService, authorizationService handlers.AuthorizationService) {
	r.Route("/v1", func(v1 chi.Router) {
		v1.Use(authMW)
		_ = authorizationService
		v1.Get("/auth-check", handlers.AuthCheckHandler)

		if bucketService != nil {
			v1.Get("/bucket-connections", handlers.ListBucketConnectionsHandler(bucketService))
		}
	})
}
