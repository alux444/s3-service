package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"s3-service/internal/database"
)

var ErrInvalidObjectReadInput = errors.New("invalid object read input")

type ObjectReader interface {
	ReadObject(ctx context.Context, input ObjectReadInput) (ObjectReadResult, error)
}

type ObjectReadInput struct {
	BucketName      string
	ObjectKey       string
	ProjectID       string
	AppID           string
	Region          string
	RoleARN         string
	ExternalID      *string
	AllowedPrefixes []string
}

type ObjectReadResult struct {
	Body          io.ReadCloser
	ContentType   string
	ContentLength int64
	ETag          string
}

type ObjectReadService struct {
	bucketRepo ObjectUploadBucketRepository
	reader     ObjectReader
}

func NewObjectReadService(bucketRepo ObjectUploadBucketRepository, reader ObjectReader) *ObjectReadService {
	return &ObjectReadService{bucketRepo: bucketRepo, reader: reader}
}

func (s *ObjectReadService) ReadObject(ctx context.Context, input ObjectReadInput) (ObjectReadResult, error) {
	slog.Info("object_read_started",
		"project_id", input.ProjectID,
		"app_id", input.AppID,
		"bucket_name", input.BucketName,
		"object_key", input.ObjectKey,
	)
	if input.ProjectID == "" || input.AppID == "" || input.BucketName == "" || input.ObjectKey == "" {
		slog.Info("object_read_invalid_input",
			"project_id", input.ProjectID,
			"app_id", input.AppID,
			"bucket_name", input.BucketName,
			"object_key", input.ObjectKey,
		)
		return ObjectReadResult{}, fmt.Errorf("%w: project_id, app_id, bucket_name and object_key are required", ErrInvalidObjectReadInput)
	}
	if s.bucketRepo == nil {
		return ObjectReadResult{}, errors.New("object read bucket repository dependency is not configured")
	}
	if s.reader == nil {
		return ObjectReadResult{}, errors.New("object reader dependency is not configured")
	}

	buckets, err := s.bucketRepo.ListActiveBucketsForConnectionScope(ctx, input.ProjectID, input.AppID)
	if err != nil {
		slog.Info("object_read_bucket_lookup_failed",
			"project_id", input.ProjectID,
			"app_id", input.AppID,
			"bucket_name", input.BucketName,
			"error", err,
		)
		return ObjectReadResult{}, fmt.Errorf("list bucket connections for read: %w", err)
	}

	var selected *database.BucketConnection
	for i := range buckets {
		if buckets[i].BucketName == input.BucketName {
			selected = &buckets[i]
			break
		}
	}
	if selected == nil {
		slog.Info("object_read_bucket_not_found",
			"project_id", input.ProjectID,
			"app_id", input.AppID,
			"bucket_name", input.BucketName,
		)
		return ObjectReadResult{}, fmt.Errorf("%w: %s", ErrBucketConnectionNotFound, input.BucketName)
	}

	readInput := input
	readInput.Region = selected.Region
	readInput.RoleARN = selected.RoleARN
	readInput.ExternalID = selected.ExternalID
	readInput.AllowedPrefixes = selected.AllowedPrefixes

	result, err := s.reader.ReadObject(ctx, readInput)
	if err != nil {
		slog.Info("object_read_upstream_failed",
			"bucket_name", input.BucketName,
			"object_key", input.ObjectKey,
			"error", err,
		)
		return ObjectReadResult{}, err
	}
	slog.Info("object_read_completed",
		"bucket_name", input.BucketName,
		"object_key", input.ObjectKey,
		"content_type", result.ContentType,
		"content_length", result.ContentLength,
	)
	return result, nil
}
