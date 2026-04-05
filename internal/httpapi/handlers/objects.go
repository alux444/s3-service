package handlers

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"

	"s3-service/internal/auth"
	"s3-service/internal/httpapi"
	"s3-service/internal/s3"
	"s3-service/internal/service"
)

type objectRequest struct {
	BucketName string `json:"bucket_name"`
	ObjectKey  string `json:"object_key"`
}

type objectUploadRequest struct {
	BucketName  string            `json:"bucket_name"`
	ObjectKey   string            `json:"object_key"`
	ContentType string            `json:"content_type"`
	ContentB64  string            `json:"content_b64"`
	Metadata    map[string]string `json:"metadata"`
}

type objectUploadResponse struct {
	Uploaded  bool   `json:"uploaded"`
	Bucket    string `json:"bucket"`
	ObjectKey string `json:"object_key"`
	ETag      string `json:"etag,omitempty"`
	Size      int64  `json:"size,omitempty"`
}

type objectDeleteResponse struct {
	Deleted   bool   `json:"deleted"`
	Bucket    string `json:"bucket"`
	ObjectKey string `json:"object_key"`
}

type ObjectUploadService interface {
	UploadObject(ctx context.Context, input service.ObjectUploadInput) (service.ObjectUploadResult, error)
}

type ObjectDeleteService interface {
	DeleteObject(ctx context.Context, input service.ObjectDeleteInput) (service.ObjectDeleteResult, error)
}

func UploadObjectHandler(authorizationService AuthorizationService, uploadService ObjectUploadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsOrUnauthorized(w, r)
		if !ok {
			return
		}

		var req objectUploadRequest
		if !decodeJSONOrBadRequest(w, r, &req) {
			return
		}

		if req.BucketName == "" || req.ObjectKey == "" || req.ContentType == "" || req.ContentB64 == "" {
			writeRequiredFieldsError(w, r, "bucket_name, object_key, content_type, and content_b64 are required", "bucket_name", "object_key", "content_type", "content_b64")
			return
		}

		decision := authorizationService.Authorize(r.Context(), auth.AuthorizationRequest{
			Claims:     claims,
			BucketName: req.BucketName,
			Action:     auth.ActionWrite,
			ObjectKey:  req.ObjectKey,
		})
		if !decision.Allowed {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "operation not permitted for this scope", httpapi.AuthDetails{Reason: decision.Reason})
			return
		}

		if uploadService == nil {
			writeUploadNotImplemented(w, r)
			return
		}

		body, err := base64.StdEncoding.DecodeString(req.ContentB64)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_request", "content_b64 must be valid base64", httpapi.ValidationDetails{Field: "content_b64", Reason: "invalid_base64"})
			return
		}

		result, err := uploadService.UploadObject(r.Context(), service.ObjectUploadInput{
			ProjectID:   claims.ProjectID,
			AppID:       claims.AppID,
			BucketName:  req.BucketName,
			ObjectKey:   req.ObjectKey,
			ContentType: req.ContentType,
			Body:        body,
			Metadata:    req.Metadata,
		})
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidObjectUploadInput), errors.Is(err, s3.ErrUnsupportedContentType), errors.Is(err, s3.ErrObjectTooLarge), errors.Is(err, s3.ErrInvalidAssumeRoleInput):
				httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_request", err.Error(), httpapi.ValidationDetails{Field: "upload", Reason: "invalid_input"})
				return
			case errors.Is(err, service.ErrBucketConnectionNotFound):
				httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "bucket connection not found for scope", httpapi.NotFoundDetails{Resource: "bucket_connection", ID: req.BucketName})
				return
			default:
				httpapi.WriteError(w, r, http.StatusBadGateway, "upstream_failure", "failed to upload object to storage provider", nil)
				return
			}
		}

		httpapi.WriteCreated(w, r, objectUploadResponse{
			Uploaded:  true,
			Bucket:    req.BucketName,
			ObjectKey: req.ObjectKey,
			ETag:      result.ETag,
			Size:      result.Size,
		})
	}
}

func writeUploadNotImplemented(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, r, http.StatusNotImplemented, "not_implemented", "upload is not implemented yet", nil)
}

func DeleteObjectHandler(authorizationService AuthorizationService, deleteService ObjectDeleteService) http.HandlerFunc {
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
			Action:     auth.ActionDelete,
			ObjectKey:  req.ObjectKey,
		})
		if !decision.Allowed {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "operation not permitted for this scope", httpapi.AuthDetails{Reason: decision.Reason})
			return
		}

		if deleteService == nil {
			httpapi.WriteError(w, r, http.StatusNotImplemented, "not_implemented", "delete is not implemented yet", nil)
			return
		}

		result, err := deleteService.DeleteObject(r.Context(), service.ObjectDeleteInput{
			ProjectID:  claims.ProjectID,
			AppID:      claims.AppID,
			BucketName: req.BucketName,
			ObjectKey:  req.ObjectKey,
		})
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidObjectDeleteInput), errors.Is(err, s3.ErrDeletePrefixGuardrailViolation), errors.Is(err, s3.ErrInvalidAssumeRoleInput):
				httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_request", err.Error(), httpapi.ValidationDetails{Field: "delete", Reason: "invalid_input"})
				return
			case errors.Is(err, service.ErrBucketConnectionNotFound):
				httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "bucket connection not found for scope", httpapi.NotFoundDetails{Resource: "bucket_connection", ID: req.BucketName})
				return
			default:
				httpapi.WriteError(w, r, http.StatusBadGateway, "upstream_failure", "failed to delete object from storage provider", nil)
				return
			}
		}

		httpapi.WriteOK(w, r, objectDeleteResponse{Deleted: result.Deleted, Bucket: req.BucketName, ObjectKey: req.ObjectKey})
	}
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
