package handlers

import (
	"net/http"

	"s3-service/internal/httpapi"
)

func AuthCheckHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}

	httpapi.WriteOK(w, r, map[string]any{
		"sub":            claims.Subject,
		"app_id":         claims.AppID,
		"project_id":     claims.ProjectID,
		"role":           claims.Role,
		"principal_type": claims.PrincipalType,
	})
}
