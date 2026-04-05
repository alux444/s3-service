package service

import (
	"context"
	"errors"
	"fmt"

	"s3-service/internal/database"
)

var ErrInvalidObjectUploadInput = errors.New("invalid object upload input")
var ErrBucketConnectionNotFound = errors.New("bucket connection not found")

type ObjectUploadBucketRepository interface {
	ListActiveBucketsForConnectionScope(ctx context.Context, projectID string, appID string) ([]database.BucketConnection, error)
}

// ObjectUploader is the boundary between service logic and S3 adapter logic.
//
// WISHFUL THINKING (3.4):
// - Add typed errors for unsupported content type and size limit exceeded.
// - Add request context fields for richer audit logging.
type ObjectUploader interface {
	UploadObject(ctx context.Context, input ObjectUploadInput) (ObjectUploadResult, error)
}

// ObjectUploadInput is service-level input for object upload.
//
// WISHFUL THINKING (3.4):
// - If upload policies vary by app, include policy scope fields here.
// - Consider adding explicit checksum and encryption intent.
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
//
// WISHFUL THINKING (3.4):
// - Resolve bucket connection metadata (region/role/external ID) by bucket_name + claim scope.
// - Enforce business rules before delegating to uploader.
// - Emit audit-friendly result details.
type ObjectUploadService struct {
	bucketRepo ObjectUploadBucketRepository
	uploader   ObjectUploader
}

func NewObjectUploadService(bucketRepo ObjectUploadBucketRepository, uploader ObjectUploader) *ObjectUploadService {
	return &ObjectUploadService{bucketRepo: bucketRepo, uploader: uploader}
}

func (s *ObjectUploadService) UploadObject(ctx context.Context, input ObjectUploadInput) (ObjectUploadResult, error) {
	if input.ProjectID == "" || input.AppID == "" || input.BucketName == "" || input.ObjectKey == "" {
		return ObjectUploadResult{}, fmt.Errorf("%w: project_id, app_id, bucket_name and object_key are required", ErrInvalidObjectUploadInput)
	}
	if len(input.Body) == 0 {
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
		return ObjectUploadResult{}, fmt.Errorf("%w: %s", ErrBucketConnectionNotFound, input.BucketName)
	}

	uploadInput := input
	uploadInput.Region = selected.Region
	uploadInput.RoleARN = selected.RoleARN
	uploadInput.ExternalID = selected.ExternalID

	return s.uploader.UploadObject(ctx, uploadInput)
}
