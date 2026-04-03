package handlers

import (
	"encoding/json"
	"net/http"

	"s3-service/internal/auth"
	"s3-service/internal/httpapi"
	"s3-service/internal/httpapi/middleware"
)

func claimsOrUnauthorized(w http.ResponseWriter, r *http.Request) (auth.Claims, bool) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeAuthRequired(w, r)
		return auth.Claims{}, false
	}
	return claims, true
}

func decodeJSONOrBadRequest(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request body", httpapi.ValidationDetails{Field: "body", Reason: "invalid_json"})
		return false
	}

	return true
}

func writeRequiredFieldsError(w http.ResponseWriter, r *http.Request, message string, fields ...string) {
	details := make([]httpapi.ValidationDetails, 0, len(fields))
	for _, field := range fields {
		details = append(details, httpapi.ValidationDetails{Field: field, Reason: "required"})
	}

	httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_request", message, httpapi.MultiValidationDetails{Errors: details})
}

func writeAuthRequired(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, r, http.StatusUnauthorized, "auth_failed", "authentication required", httpapi.AuthDetails{Reason: "missing"})
}
