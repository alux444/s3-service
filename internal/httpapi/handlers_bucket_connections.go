package httpapi

import "net/http"

func listBucketConnectionsHandler(bucketService BucketConnectionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, r, http.StatusUnauthorized, "auth_failed", "authentication required", AuthDetails{Reason: "missing"})
			return
		}

		buckets, err := bucketService.ListForScope(r.Context(), claims.ProjectID, claims.AppID)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "list_buckets_failed", "failed to list bucket connections", nil)
			return
		}

		writeOK(w, r, map[string]any{
			"buckets": buckets,
		})
	}
}
