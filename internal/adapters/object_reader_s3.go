package adapters

import (
	"context"

	"s3-service/internal/s3"
	"s3-service/internal/service"
)

type objectReaderHelper interface {
	GetObject(ctx context.Context, input s3.GetObjectInput) (s3.GetObjectResult, error)
}

type S3ObjectReaderAdapter struct {
	helper objectReaderHelper
}

func NewS3ObjectReaderAdapter(helper *s3.GetHelper) *S3ObjectReaderAdapter {
	return &S3ObjectReaderAdapter{helper: helper}
}

func (a *S3ObjectReaderAdapter) ReadObject(ctx context.Context, input service.ObjectReadInput) (service.ObjectReadResult, error) {
	result, err := a.helper.GetObject(ctx, s3.GetObjectInput{
		BucketName:      input.BucketName,
		ObjectKey:       input.ObjectKey,
		Region:          input.Region,
		RoleARN:         input.RoleARN,
		ExternalID:      input.ExternalID,
		AllowedPrefixes: input.AllowedPrefixes,
	})
	if err != nil {
		return service.ObjectReadResult{}, err
	}

	return service.ObjectReadResult{
		Body:          result.Body,
		ContentType:   result.ContentType,
		ContentLength: result.ContentLength,
		ETag:          result.ETag,
	}, nil
}
