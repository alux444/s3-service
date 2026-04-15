package handlers

import (
	"errors"
	"net/http"

	"s3-service/internal/auth"
	"s3-service/internal/httpapi"
	"s3-service/internal/service"
)

type upsertAccessPolicyRequest struct {
	BucketName      string   `json:"bucket_name"`
	PrincipalType   string   `json:"principal_type"`
	PrincipalID     string   `json:"principal_id"`
	Role            string   `json:"role"`
	CanRead         *bool    `json:"can_read"`
	CanWrite        *bool    `json:"can_write"`
	CanDelete       *bool    `json:"can_delete"`
	CanList         *bool    `json:"can_list"`
	PrefixAllowlist []string `json:"prefix_allowlist"`
}

type upsertAccessPolicyResponse struct {
	Upserted bool `json:"upserted"`
}

func UpsertAccessPolicyHandler(bucketService BucketConnectionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsOrUnauthorized(w, r)
		if !ok {
			return
		}

		if claims.Role != auth.RoleAdmin {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "only admin role can manage access policies", httpapi.AuthDetails{Reason: "role_required"})
			return
		}

		var req upsertAccessPolicyRequest
		if !decodeJSONOrBadRequest(w, r, &req) {
			return
		}

		requiredFields := make([]string, 0, 4)
		if req.BucketName == "" {
			requiredFields = append(requiredFields, "bucket_name")
		}
		if req.PrincipalType == "" {
			requiredFields = append(requiredFields, "principal_type")
		}
		if req.PrincipalID == "" {
			requiredFields = append(requiredFields, "principal_id")
		}
		if req.Role == "" {
			requiredFields = append(requiredFields, "role")
		}
		if len(requiredFields) > 0 {
			writeRequiredFieldsError(w, r, "bucket_name, principal_type, principal_id, and role are required", requiredFields...)
			return
		}

		if _, err := auth.ParsePrincipalType(req.PrincipalType); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_request", "principal_type must be one of: user, service", httpapi.ValidationDetails{Field: "principal_type", Reason: "invalid_value"})
			return
		}

		if _, err := auth.ParseRole(req.Role); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_request", "role must be one of: admin, project-client, read-only-client", httpapi.ValidationDetails{Field: "role", Reason: "invalid_value"})
			return
		}

		canRead := boolOrDefault(req.CanRead, true)
		canWrite := boolOrDefault(req.CanWrite, false)
		canDelete := boolOrDefault(req.CanDelete, false)
		canList := boolOrDefault(req.CanList, true)

		err := bucketService.UpsertAccessPolicyForScope(
			r.Context(),
			claims.ProjectID,
			claims.AppID,
			req.BucketName,
			req.PrincipalType,
			req.PrincipalID,
			req.Role,
			canRead,
			canWrite,
			canDelete,
			canList,
			req.PrefixAllowlist,
		)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidAccessPolicyInput):
				httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_request", err.Error(), httpapi.ValidationDetails{Field: "access_policy", Reason: "invalid_input"})
				return
			case errors.Is(err, service.ErrBucketConnectionNotFound):
				httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "bucket connection not found for scope", httpapi.NotFoundDetails{Resource: "bucket_connection", ID: req.BucketName})
				return
			default:
				httpapi.WriteError(w, r, http.StatusInternalServerError, "upsert_access_policy_failed", "failed to create or update access policy", nil)
				return
			}
		}

		httpapi.WriteOK(w, r, upsertAccessPolicyResponse{Upserted: true})
	}
}

func boolOrDefault(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}
