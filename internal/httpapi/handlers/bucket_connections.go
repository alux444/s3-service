package handlers

import (
	"context"
	"net/http"

	"s3-service/internal/httpapi"
	"s3-service/internal/httpapi/middleware"
)

type BucketConnectionService interface {
	ListForScope(ctx context.Context, projectID, appID string) ([]string, error)
}

func ListBucketConnectionsHandler(bucketService BucketConnectionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "auth_failed", "authentication required", httpapi.AuthDetails{Reason: "missing"})
			return
		}

		buckets, err := bucketService.ListForScope(r.Context(), claims.ProjectID, claims.AppID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "list_buckets_failed", "failed to list bucket connections", nil)
			return
		}

		httpapi.WriteOK(w, r, map[string]any{
			"buckets": buckets,
		})
	}
}
