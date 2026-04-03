package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"s3-service/internal/database"
	"s3-service/internal/httpapi"
	"s3-service/internal/httpapi/middleware"
	"s3-service/internal/service"
)

type BucketConnectionService interface {
	ListForScope(ctx context.Context, projectID, appID string) ([]string, error)
	CreateForScope(ctx context.Context, projectID string, appID string, bucketName string, region string, roleARN string, externalID *string, allowedPrefixes []string) error
}

type createBucketConnectionRequest struct {
	BucketName      string   `json:"bucket_name"`
	Region          string   `json:"region"`
	RoleARN         string   `json:"role_arn"`
	ExternalID      *string  `json:"external_id"`
	AllowedPrefixes []string `json:"allowed_prefixes"`
}

func CreateBucketConnectionHandler(bucketService BucketConnectionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			// TODO(cleanup): centralize repeated auth-missing error response used across handlers.
			httpapi.WriteError(w, r, http.StatusUnauthorized, "auth_failed", "authentication required", httpapi.AuthDetails{Reason: "missing"})
			return
		}

		var req createBucketConnectionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// TODO(cleanup): move repeated invalid-JSON response into a shared transport helper.
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request body", httpapi.ValidationDetails{Field: "body", Reason: "invalid_json"})
			return
		}

		err := bucketService.CreateForScope(
			r.Context(),
			claims.ProjectID,
			claims.AppID,
			req.BucketName,
			req.Region,
			req.RoleARN,
			req.ExternalID,
			req.AllowedPrefixes,
		)
		if err != nil {
			if errors.Is(err, service.ErrInvalidBucketConnectionInput) {
				// TODO(cleanup): standardize required-field error builders to avoid duplicate field lists.
				httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_request", "bucket_name, region, and role_arn are required", httpapi.MultiValidationDetails{Errors: []httpapi.ValidationDetails{{Field: "bucket_name", Reason: "required"}, {Field: "region", Reason: "required"}, {Field: "role_arn", Reason: "required"}}})
				return
			}
			if errors.Is(err, database.ErrBucketConnectionAlreadyExists) {
				httpapi.WriteError(w, r, http.StatusConflict, "bucket_connection_exists", "bucket connection already exists for this project/app scope", httpapi.ConflictDetails{Resource: "bucket_connection", Field: "bucket_name", Value: req.BucketName})
				return
			}
			httpapi.WriteError(w, r, http.StatusInternalServerError, "create_bucket_connection_failed", "failed to create bucket connection", nil)
			return
		}

		httpapi.WriteCreated(w, r, map[string]any{"created": true})
	}
}

func ListBucketConnectionsHandler(bucketService BucketConnectionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			// TODO(cleanup): centralize repeated auth-missing error response used across handlers.
			httpapi.WriteError(w, r, http.StatusUnauthorized, "auth_failed", "authentication required", httpapi.AuthDetails{Reason: "missing"})
			return
		}

		buckets, err := bucketService.ListForScope(r.Context(), claims.ProjectID, claims.AppID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "list_buckets_failed", "failed to list bucket connections", nil)
			return
		}

		httpapi.WriteOK(w, r, map[string]any{
			"buckets": buckets,
		})
	}
}
