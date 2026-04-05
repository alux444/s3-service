package adapters

import (
	"context"

	"s3-service/internal/s3"
	"s3-service/internal/service"
)

type objectDeleterHelper interface {
	DeleteObject(ctx context.Context, input s3.DeleteObjectInput) (s3.DeleteObjectResult, error)
}

type S3ObjectDeleterAdapter struct {
	helper objectDeleterHelper
}

func NewS3ObjectDeleterAdapter(helper *s3.DeleteHelper) *S3ObjectDeleterAdapter {
	return &S3ObjectDeleterAdapter{helper: helper}
}

func (a *S3ObjectDeleterAdapter) DeleteObject(ctx context.Context, input service.ObjectDeleteInput) (service.ObjectDeleteResult, error) {
	result, err := a.helper.DeleteObject(ctx, s3.DeleteObjectInput{
		BucketName:      input.BucketName,
		ObjectKey:       input.ObjectKey,
		Region:          input.Region,
		RoleARN:         input.RoleARN,
		ExternalID:      input.ExternalID,
		AllowedPrefixes: input.AllowedPrefixes,
	})
	if err != nil {
		return service.ObjectDeleteResult{}, err
	}

	return service.ObjectDeleteResult{Deleted: result.Deleted}, nil
}
