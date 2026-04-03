package handlers

import (
	"net/http"

	"s3-service/internal/auth"
	"s3-service/internal/httpapi"
)

type authCheckResponse struct {
	Subject       string             `json:"sub"`
	AppID         string             `json:"app_id"`
	ProjectID     string             `json:"project_id"`
	Role          auth.Role          `json:"role"`
	PrincipalType auth.PrincipalType `json:"principal_type"`
}

func AuthCheckHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}

	httpapi.WriteOK(w, r, authCheckResponse{
		Subject:       claims.Subject,
		AppID:         claims.AppID,
		ProjectID:     claims.ProjectID,
		Role:          claims.Role,
		PrincipalType: claims.PrincipalType,
	})
}
