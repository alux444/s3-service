package adapters

import (
	"context"

	"s3-service/internal/s3"
	"s3-service/internal/service"
)

type objectUploaderHelper interface {
	UploadObject(ctx context.Context, input s3.UploadObjectInput) (s3.UploadObjectResult, error)
}

type S3ObjectUploaderAdapter struct {
	helper objectUploaderHelper
}

func NewS3ObjectUploaderAdapter(helper *s3.UploadHelper) *S3ObjectUploaderAdapter {
	return &S3ObjectUploaderAdapter{helper: helper}
}

func (a *S3ObjectUploaderAdapter) UploadObject(ctx context.Context, input service.ObjectUploadInput) (service.ObjectUploadResult, error) {
	result, err := a.helper.UploadObject(ctx, s3.UploadObjectInput{
		BucketName:  input.BucketName,
		ObjectKey:   input.ObjectKey,
		Region:      input.Region,
		RoleARN:     input.RoleARN,
		ExternalID:  input.ExternalID,
		ContentType: input.ContentType,
		Body:        input.Body,
		Metadata:    input.Metadata,
	})
	if err != nil {
		return service.ObjectUploadResult{}, err
	}

	return service.ObjectUploadResult{ETag: result.ETag, Size: result.Size}, nil
}
