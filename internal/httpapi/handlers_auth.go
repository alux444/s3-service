package httpapi

import "net/http"

func authCheckHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "auth_failed", "authentication required", AuthDetails{Reason: "missing"})
		return
	}

	writeOK(w, r, map[string]any{
		"sub":        claims.Subject,
		"app_id":     claims.AppID,
		"project_id": claims.ProjectID,
		"role":       claims.Role,
	})
}
