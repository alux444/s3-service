package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func registerV1Routes(r chi.Router, authMW func(http.Handler) http.Handler, bucketService BucketConnectionService) {
	r.Route("/v1", func(v1 chi.Router) {
		v1.Use(authMW)
		v1.Get("/auth-check", authCheckHandler)

		if bucketService != nil {
			v1.Get("/bucket-connections", listBucketConnectionsHandler(bucketService))
		}
	})
}
