package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"s3-service/internal/database"
)

var ErrInvalidObjectUploadInput = errors.New("invalid object upload input")
var ErrBucketConnectionNotFound = errors.New("bucket connection not found")

type ObjectUploadBucketRepository interface {
	ListActiveBucketsForConnectionScope(ctx context.Context, projectID string, appID string) ([]database.BucketConnection, error)
}

// ObjectUploader is the boundary between service logic and S3 adapter logic.
type ObjectUploader interface {
	UploadObject(ctx context.Context, input ObjectUploadInput) (ObjectUploadResult, error)
}

// ObjectUploadInput is service-level input for object upload.
type ObjectUploadInput struct {
	BucketName  string
	ObjectKey   string
	ProjectID   string
	AppID       string
	Region      string
	RoleARN     string
	ExternalID  *string
	ContentType string
	Body        []byte
	Metadata    map[string]string
}

type ObjectUploadResult struct {
	ETag string
	Size int64
}

// ObjectUploadService orchestrates business-level upload flow.
type ObjectUploadService struct {
	bucketRepo ObjectUploadBucketRepository
	uploader   ObjectUploader
}

func NewObjectUploadService(bucketRepo ObjectUploadBucketRepository, uploader ObjectUploader) *ObjectUploadService {
	return &ObjectUploadService{bucketRepo: bucketRepo, uploader: uploader}
}

func (s *ObjectUploadService) UploadObject(ctx context.Context, input ObjectUploadInput) (ObjectUploadResult, error) {
	slog.Info("object_upload_started",
		"project_id", input.ProjectID,
		"app_id", input.AppID,
		"bucket_name", input.BucketName,
		"object_key", input.ObjectKey,
		"content_type", input.ContentType,
		"body_size", len(input.Body),
	)
	if input.ProjectID == "" || input.AppID == "" || input.BucketName == "" || input.ObjectKey == "" {
		slog.Info("object_upload_invalid_input",
			"project_id", input.ProjectID,
			"app_id", input.AppID,
			"bucket_name", input.BucketName,
			"object_key", input.ObjectKey,
		)
		return ObjectUploadResult{}, fmt.Errorf("%w: project_id, app_id, bucket_name and object_key are required", ErrInvalidObjectUploadInput)
	}
	if len(input.Body) == 0 {
		slog.Info("object_upload_invalid_body_empty",
			"bucket_name", input.BucketName,
			"object_key", input.ObjectKey,
		)
		return ObjectUploadResult{}, fmt.Errorf("%w: body is required", ErrInvalidObjectUploadInput)
	}
	if s.bucketRepo == nil {
		return ObjectUploadResult{}, errors.New("object upload bucket repository dependency is not configured")
	}
	if s.uploader == nil {
		return ObjectUploadResult{}, errors.New("object uploader dependency is not configured")
	}

	buckets, err := s.bucketRepo.ListActiveBucketsForConnectionScope(ctx, input.ProjectID, input.AppID)
	if err != nil {
		slog.Info("object_upload_bucket_lookup_failed",
			"project_id", input.ProjectID,
			"app_id", input.AppID,
			"bucket_name", input.BucketName,
			"error", err,
		)
		return ObjectUploadResult{}, fmt.Errorf("list bucket connections for upload: %w", err)
	}

	var selected *database.BucketConnection
	for i := range buckets {
		if buckets[i].BucketName == input.BucketName {
			selected = &buckets[i]
			break
		}
	}
	if selected == nil {
		slog.Info("object_upload_bucket_not_found",
			"project_id", input.ProjectID,
			"app_id", input.AppID,
			"bucket_name", input.BucketName,
		)
		return ObjectUploadResult{}, fmt.Errorf("%w: %s", ErrBucketConnectionNotFound, input.BucketName)
	}

	uploadInput := input
	uploadInput.Region = selected.Region
	uploadInput.RoleARN = selected.RoleARN
	uploadInput.ExternalID = selected.ExternalID

	result, err := s.uploader.UploadObject(ctx, uploadInput)
	if err != nil {
		slog.Info("object_upload_upstream_failed",
			"bucket_name", input.BucketName,
			"object_key", input.ObjectKey,
			"error", err,
		)
		return ObjectUploadResult{}, err
	}
	slog.Info("object_upload_completed",
		"bucket_name", input.BucketName,
		"object_key", input.ObjectKey,
		"size", result.Size,
		"etag", result.ETag,
	)
	return result, nil
}
