package handlers

import (
	"context"
	"errors"
	"net/http"

	"s3-service/internal/database"
	"s3-service/internal/httpapi"
	"s3-service/internal/s3"
	"s3-service/internal/service"
)

type BucketConnectionService interface {
	ListForScope(ctx context.Context, projectID, appID string) ([]database.BucketConnection, error)
	CreateForScope(ctx context.Context, projectID string, appID string, bucketName string, region string, roleARN string, externalID *string, allowedPrefixes []string) error
}

type createBucketConnectionRequest struct {
	BucketName      string   `json:"bucket_name"`
	Region          string   `json:"region"`
	RoleARN         string   `json:"role_arn"`
	ExternalID      *string  `json:"external_id"`
	AllowedPrefixes []string `json:"allowed_prefixes"`
}

type createBucketConnectionResponse struct {
	Created bool `json:"created"`
}

type listBucketConnectionsResponse struct {
	Buckets []database.BucketConnection `json:"buckets"`
}

func CreateBucketConnectionHandler(bucketService BucketConnectionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsOrUnauthorized(w, r)
		if !ok {
			return
		}

		var req createBucketConnectionRequest
		if !decodeJSONOrBadRequest(w, r, &req) {
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
				writeRequiredFieldsError(w, r, "bucket_name, region, and role_arn are required", "bucket_name", "region", "role_arn")
				return
			}
			if errors.Is(err, s3.ErrBucketSecurityBaselineViolation) {
				httpapi.WriteError(w, r, http.StatusBadRequest, "bucket_security_baseline_failed", err.Error(), httpapi.ValidationDetails{Field: "bucket_name", Reason: "bucket must be private and enforce bucket owner object ownership"})
				return
			}
			if errors.Is(err, database.ErrBucketConnectionAlreadyExists) {
				httpapi.WriteError(w, r, http.StatusConflict, "bucket_connection_exists", "bucket connection already exists for this project/app scope", httpapi.ConflictDetails{Resource: "bucket_connection", Field: "bucket_name", Value: req.BucketName})
				return
			}
			httpapi.WriteError(w, r, http.StatusInternalServerError, "create_bucket_connection_failed", "failed to create bucket connection", nil)
			return
		}

		httpapi.WriteCreated(w, r, createBucketConnectionResponse{Created: true})
	}
}

func ListBucketConnectionsHandler(bucketService BucketConnectionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsOrUnauthorized(w, r)
		if !ok {
			return
		}

		buckets, err := bucketService.ListForScope(r.Context(), claims.ProjectID, claims.AppID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "list_buckets_failed", "failed to list bucket connections", nil)
			return
		}

		httpapi.WriteOK(w, r, listBucketConnectionsResponse{Buckets: buckets})
	}
}
