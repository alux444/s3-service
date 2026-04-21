package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"s3-service/internal/auth"
	"s3-service/internal/httpapi"
	"s3-service/internal/httpapi/middleware"
)

func claimsOrUnauthorized(w http.ResponseWriter, r *http.Request) (auth.Claims, bool) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		slog.Info("handler_auth_claims_missing",
			"method", r.Method,
			"path", r.URL.Path,
		)
		writeAuthRequired(w, r)
		return auth.Claims{}, false
	}
	slog.Info("handler_auth_claims_loaded",
		"method", r.Method,
		"path", r.URL.Path,
		"principal_type", claims.PrincipalType,
		"subject", claims.Subject,
		"project_id", claims.ProjectID,
		"app_id", claims.AppID,
		"role", claims.Role,
	)
	return claims, true
}

func decodeJSONOrBadRequest[T any](w http.ResponseWriter, r *http.Request, dst *T) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		slog.Info("handler_decode_json_failed",
			"method", r.Method,
			"path", r.URL.Path,
			"error", err,
		)
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request body", httpapi.ValidationDetails{Field: "body", Reason: "invalid_json"})
		return false
	}
	slog.Info("handler_decode_json_completed",
		"method", r.Method,
		"path", r.URL.Path,
	)

	return true
}

func writeRequiredFieldsError(w http.ResponseWriter, r *http.Request, message string, fields ...string) {
	slog.Info("handler_required_fields_missing",
		"method", r.Method,
		"path", r.URL.Path,
		"fields", fields,
	)
	details := make([]httpapi.ValidationDetails, 0, len(fields))
	for _, field := range fields {
		details = append(details, httpapi.ValidationDetails{Field: field, Reason: "required"})
	}

	httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_request", message, httpapi.MultiValidationDetails{Errors: details})
}

func writeAuthRequired(w http.ResponseWriter, r *http.Request) {
	slog.Info("handler_auth_required",
		"method", r.Method,
		"path", r.URL.Path,
	)
	httpapi.WriteError(w, r, http.StatusUnauthorized, "auth_failed", "authentication required", httpapi.AuthDetails{Reason: "missing"})
}
