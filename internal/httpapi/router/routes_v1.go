package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"s3-service/internal/httpapi/handlers"
)

func registerV1Routes(r chi.Router, authMW func(http.Handler) http.Handler, auditMW func(http.Handler) http.Handler, rateLimitMW func(http.Handler) http.Handler, bucketService handlers.BucketConnectionService, authorizationService handlers.AuthorizationService, objectUploadService handlers.ObjectUploadService, objectDeleteService handlers.ObjectDeleteService, objectPresignService handlers.ObjectPresignService, objectListService handlers.ObjectListService, objectReadService handlers.ObjectReadService) {
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
			v1.Post("/bucket-connections", handlers.CreateBucketConnectionHandler(bucketService))
			v1.Get("/bucket-connections", handlers.ListBucketConnectionsHandler(bucketService))
		}
		if authorizationService != nil {
			v1.Post("/objects/upload", handlers.UploadObjectHandler(authorizationService, objectUploadService))
			v1.Delete("/objects", handlers.DeleteObjectHandler(authorizationService, objectDeleteService))
			v1.Post("/objects/presign-upload", handlers.PresignUploadObjectHandlerWithService(authorizationService, objectPresignService))
			v1.Post("/objects/presign-download", handlers.PresignDownloadObjectHandlerWithService(authorizationService, objectPresignService))
			v1.Get("/images", handlers.ListImagesHandler(objectListService))
			v1.Get("/images/{id}", handlers.GetImageHandler(authorizationService, objectReadService))
			v1.Delete("/images/{id}", handlers.DeleteImageHandler(authorizationService, objectDeleteService))
		}
	})
}
