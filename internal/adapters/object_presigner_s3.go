package adapters

import (
	"context"

	"s3-service/internal/s3"
	"s3-service/internal/service"
)

type S3ObjectPresignerAdapter struct {
	helper *s3.PresignHelper
}

func NewS3ObjectPresignerAdapter(helper *s3.PresignHelper) *S3ObjectPresignerAdapter {
	return &S3ObjectPresignerAdapter{helper: helper}
}

func (a *S3ObjectPresignerAdapter) PresignObject(ctx context.Context, input service.ObjectPresignInput) (service.ObjectPresignResult, error) {
	result, err := a.helper.PresignObject(ctx, s3.PresignObjectInput{
		BucketName:  input.BucketName,
		ObjectKey:   input.ObjectKey,
		Region:      input.Region,
		RoleARN:     input.RoleARN,
		ExternalID:  input.ExternalID,
		Method:      input.Method,
		ExpiresIn:   input.ExpiresIn,
		ContentType: input.ContentType,
	})
	if err != nil {
		return service.ObjectPresignResult{}, err
	}

	return service.ObjectPresignResult{
		URL:       result.URL,
		Method:    result.Method,
		ExpiresIn: result.ExpiresIn,
	}, nil
}
