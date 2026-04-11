package adapters

import (
	"context"

	"s3-service/internal/s3"
	"s3-service/internal/service"
)

type objectListerHelper interface {
	ListObjects(ctx context.Context, input s3.ListObjectsInput) ([]s3.ListedObject, error)
}

type S3ObjectListerAdapter struct {
	helper objectListerHelper
}

func NewS3ObjectListerAdapter(helper *s3.ListHelper) *S3ObjectListerAdapter {
	return &S3ObjectListerAdapter{helper: helper}
}

func (a *S3ObjectListerAdapter) ListObjects(ctx context.Context, input service.ObjectListRequest) ([]service.ObjectListObject, error) {
	objects, err := a.helper.ListObjects(ctx, s3.ListObjectsInput{
		BucketName: input.BucketName,
		Prefix:     input.Prefix,
		Region:     input.Region,
		RoleARN:    input.RoleARN,
		ExternalID: input.ExternalID,
	})
	if err != nil {
		return nil, err
	}

	result := make([]service.ObjectListObject, 0, len(objects))
	for _, item := range objects {
		result = append(result, service.ObjectListObject{
			ObjectKey:    item.ObjectKey,
			Size:         item.Size,
			ETag:         item.ETag,
			LastModified: item.LastModified,
		})
	}
	return result, nil
}
