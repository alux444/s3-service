package handlers

import (
	"net/http"

	"s3-service/internal/auth"
	"s3-service/internal/httpapi"
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
		claims, ok := claimsOrUnauthorized(w, r)
		if !ok {
			return
		}

		var req objectRequest
		if !decodeJSONOrBadRequest(w, r, &req) {
			return
		}

		if req.BucketName == "" || req.ObjectKey == "" {
			writeRequiredFieldsError(w, r, "bucket_name and object_key are required", "bucket_name", "object_key")
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

		// TODO(cleanup): keep these routes exposed for contract testing until storage orchestration is implemented.
		httpapi.WriteError(w, r, http.StatusNotImplemented, "not_implemented", operation+" is not implemented yet", nil)
	}
}
