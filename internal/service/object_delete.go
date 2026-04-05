package service

import (
	"context"
	"errors"
	"fmt"

	"s3-service/internal/database"
)

var ErrInvalidObjectDeleteInput = errors.New("invalid object delete input")

type ObjectDeleter interface {
	DeleteObject(ctx context.Context, input ObjectDeleteInput) (ObjectDeleteResult, error)
}

type ObjectDeleteInput struct {
	BucketName      string
	ObjectKey       string
	ProjectID       string
	AppID           string
	Region          string
	RoleARN         string
	ExternalID      *string
	AllowedPrefixes []string
}

type ObjectDeleteResult struct {
	Deleted bool
}

type ObjectDeleteService struct {
	bucketRepo ObjectUploadBucketRepository
	deleter    ObjectDeleter
}

func NewObjectDeleteService(bucketRepo ObjectUploadBucketRepository, deleter ObjectDeleter) *ObjectDeleteService {
	return &ObjectDeleteService{bucketRepo: bucketRepo, deleter: deleter}
}

func (s *ObjectDeleteService) DeleteObject(ctx context.Context, input ObjectDeleteInput) (ObjectDeleteResult, error) {
	if input.ProjectID == "" || input.AppID == "" || input.BucketName == "" || input.ObjectKey == "" {
		return ObjectDeleteResult{}, fmt.Errorf("%w: project_id, app_id, bucket_name and object_key are required", ErrInvalidObjectDeleteInput)
	}
	if s.bucketRepo == nil {
		return ObjectDeleteResult{}, errors.New("object delete bucket repository dependency is not configured")
	}
	if s.deleter == nil {
		return ObjectDeleteResult{}, errors.New("object deleter dependency is not configured")
	}

	buckets, err := s.bucketRepo.ListActiveBucketsForConnectionScope(ctx, input.ProjectID, input.AppID)
	if err != nil {
		return ObjectDeleteResult{}, fmt.Errorf("list bucket connections for delete: %w", err)
	}

	var selected *database.BucketConnection
	for i := range buckets {
		if buckets[i].BucketName == input.BucketName {
			selected = &buckets[i]
			break
		}
	}
	if selected == nil {
		return ObjectDeleteResult{}, fmt.Errorf("%w: %s", ErrBucketConnectionNotFound, input.BucketName)
	}

	result, err := s.deleter.DeleteObject(ctx, ObjectDeleteInput{
		BucketName:      input.BucketName,
		ObjectKey:       input.ObjectKey,
		ProjectID:       input.ProjectID,
		AppID:           input.AppID,
		Region:          selected.Region,
		RoleARN:         selected.RoleARN,
		ExternalID:      selected.ExternalID,
		AllowedPrefixes: selected.AllowedPrefixes,
	})
	if err != nil {
		return ObjectDeleteResult{}, err
	}
	return result, nil
}
