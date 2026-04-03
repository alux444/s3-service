package handlers

import (
	"encoding/json"
	"net/http"

	"s3-service/internal/auth"
	"s3-service/internal/httpapi"
	"s3-service/internal/httpapi/middleware"
)

type objectRequest struct {
	BucketName string `json:"bucket_name"`
	ObjectKey  string `json:"object_key"`
}

func UploadObjectHandler(authorizationService AuthorizationService) http.HandlerFunc {
	return objectOperationHandler(authorizationService, auth.ActionWrite, "upload")
}

func DeleteObjectHandler(authorizationService AuthorizationService) http.HandlerFunc {
	return objectOperationHandler(authorizationService, auth.ActionDelete, "delete")
}

func PresignUploadObjectHandler(authorizationService AuthorizationService) http.HandlerFunc {
	return objectOperationHandler(authorizationService, auth.ActionWrite, "presign_upload")
}

func PresignDownloadObjectHandler(authorizationService AuthorizationService) http.HandlerFunc {
	return objectOperationHandler(authorizationService, auth.ActionRead, "presign_download")
}

func objectOperationHandler(authorizationService AuthorizationService, action auth.Action, operation string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "auth_failed", "authentication required", httpapi.AuthDetails{Reason: "missing"})
			return
		}

		var req objectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request body", httpapi.ValidationDetails{Field: "body", Reason: "invalid_json"})
			return
		}

		if req.BucketName == "" || req.ObjectKey == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_request", "bucket_name and object_key are required", httpapi.MultiValidationDetails{Errors: []httpapi.ValidationDetails{{Field: "bucket_name", Reason: "required"}, {Field: "object_key", Reason: "required"}}})
			return
		}

		decision := authorizationService.Authorize(r.Context(), auth.AuthorizationRequest{
			Claims:     claims,
			BucketName: req.BucketName,
			Action:     action,
			ObjectKey:  req.ObjectKey,
		})
		if !decision.Allowed {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "operation not permitted for this scope", httpapi.AuthDetails{Reason: decision.Reason})
			return
		}

		httpapi.WriteError(w, r, http.StatusNotImplemented, "not_implemented", operation+" is not implemented yet", nil)
	}
}
