package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

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
	slog.Info("object_delete_started",
		"project_id", input.ProjectID,
		"app_id", input.AppID,
		"bucket_name", input.BucketName,
		"object_key", input.ObjectKey,
	)
	if input.ProjectID == "" || input.AppID == "" || input.BucketName == "" || input.ObjectKey == "" {
		slog.Info("object_delete_invalid_input",
			"project_id", input.ProjectID,
			"app_id", input.AppID,
			"bucket_name", input.BucketName,
			"object_key", input.ObjectKey,
		)
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
		slog.Info("object_delete_bucket_lookup_failed",
			"project_id", input.ProjectID,
			"app_id", input.AppID,
			"bucket_name", input.BucketName,
			"error", err,
		)
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
		slog.Info("object_delete_bucket_not_found",
			"project_id", input.ProjectID,
			"app_id", input.AppID,
			"bucket_name", input.BucketName,
		)
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
		slog.Info("object_delete_upstream_failed",
			"bucket_name", input.BucketName,
			"object_key", input.ObjectKey,
			"error", err,
		)
		return ObjectDeleteResult{}, err
	}
	slog.Info("object_delete_completed",
		"bucket_name", input.BucketName,
		"object_key", input.ObjectKey,
		"deleted", result.Deleted,
	)
	return result, nil
}
