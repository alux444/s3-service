package handlers

import (
	"net/http"

	"s3-service/internal/httpapi"
	"s3-service/internal/httpapi/middleware"
)

func AuthCheckHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "auth_failed", "authentication required", httpapi.AuthDetails{Reason: "missing"})
		return
	}

	httpapi.WriteOK(w, r, map[string]any{
		"sub":        claims.Subject,
		"app_id":     claims.AppID,
		"project_id": claims.ProjectID,
		"role":       claims.Role,
	})
}
