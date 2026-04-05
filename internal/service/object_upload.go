package service

import (
	"context"
	"errors"
	"fmt"
)

var ErrInvalidObjectUploadInput = errors.New("invalid object upload input")

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
	uploader ObjectUploader
}

func NewObjectUploadService(uploader ObjectUploader) *ObjectUploadService {
	return &ObjectUploadService{uploader: uploader}
}

func (s *ObjectUploadService) UploadObject(ctx context.Context, input ObjectUploadInput) (ObjectUploadResult, error) {
	if input.BucketName == "" || input.ObjectKey == "" {
		return ObjectUploadResult{}, fmt.Errorf("%w: bucket_name and object_key are required", ErrInvalidObjectUploadInput)
	}
	if len(input.Body) == 0 {
		return ObjectUploadResult{}, fmt.Errorf("%w: body is required", ErrInvalidObjectUploadInput)
	}
	if s.uploader == nil {
		return ObjectUploadResult{}, errors.New("object uploader dependency is not configured")
	}

	return s.uploader.UploadObject(ctx, input)
}
