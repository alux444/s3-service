package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"s3-service/internal/database"
)

var ErrInvalidObjectListInput = errors.New("invalid object list input")

type ObjectListPolicyRepository interface {
	GetEffectiveAuthorizationPolicy(ctx context.Context, lookup database.AuthorizationPolicyLookup) (database.EffectiveAuthorizationPolicy, error)
}

type ObjectListRequest struct {
	BucketName string
	Region     string
	RoleARN    string
	ExternalID *string
	Prefix     string
}

type ObjectListObject struct {
	ObjectKey    string
	Size         int64
	ETag         string
	LastModified time.Time
}

type ObjectLister interface {
	ListObjects(ctx context.Context, input ObjectListRequest) ([]ObjectListObject, error)
}

type ObjectListInput struct {
	ProjectID     string
	AppID         string
	PrincipalType string
	PrincipalID   string
}

type ObjectListEntry struct {
	BucketName   string
	ObjectKey    string
	Size         int64
	ETag         string
	LastModified time.Time
}

type ObjectListService struct {
	bucketRepo ObjectUploadBucketRepository
	policyRepo ObjectListPolicyRepository
	lister     ObjectLister
}

func NewObjectListService(bucketRepo ObjectUploadBucketRepository, policyRepo ObjectListPolicyRepository, lister ObjectLister) *ObjectListService {
	return &ObjectListService{bucketRepo: bucketRepo, policyRepo: policyRepo, lister: lister}
}

func (s *ObjectListService) ListImages(ctx context.Context, input ObjectListInput) ([]ObjectListEntry, error) {
	slog.Info("object_list_started",
		"project_id", input.ProjectID,
		"app_id", input.AppID,
		"principal_type", input.PrincipalType,
		"principal_id", input.PrincipalID,
	)
	if input.ProjectID == "" || input.AppID == "" || input.PrincipalType == "" || input.PrincipalID == "" {
		slog.Info("object_list_invalid_input",
			"project_id", input.ProjectID,
			"app_id", input.AppID,
			"principal_type", input.PrincipalType,
			"principal_id", input.PrincipalID,
		)
		return nil, fmt.Errorf("%w: project_id, app_id, principal_type, and principal_id are required", ErrInvalidObjectListInput)
	}
	if s.bucketRepo == nil {
		return nil, errors.New("object list bucket repository dependency is not configured")
	}
	if s.policyRepo == nil {
		return nil, errors.New("object list policy repository dependency is not configured")
	}
	if s.lister == nil {
		return nil, errors.New("object list lister dependency is not configured")
	}

	buckets, err := s.bucketRepo.ListActiveBucketsForConnectionScope(ctx, input.ProjectID, input.AppID)
	if err != nil {
		slog.Info("object_list_bucket_lookup_failed",
			"project_id", input.ProjectID,
			"app_id", input.AppID,
			"error", err,
		)
		return nil, fmt.Errorf("list bucket connections for image list: %w", err)
	}
	slog.Info("object_list_bucket_lookup_completed",
		"project_id", input.ProjectID,
		"app_id", input.AppID,
		"bucket_count", len(buckets),
	)

	seen := make(map[string]struct{})
	entries := make([]ObjectListEntry, 0)

	for _, bucket := range buckets {
		policy, err := s.policyRepo.GetEffectiveAuthorizationPolicy(ctx, database.AuthorizationPolicyLookup{
			ProjectID:     input.ProjectID,
			AppID:         input.AppID,
			PrincipalType: input.PrincipalType,
			PrincipalID:   input.PrincipalID,
			BucketName:    bucket.BucketName,
		})
		if err != nil {
			if errors.Is(err, database.ErrPolicyNotFound) {
				slog.Info("object_list_policy_missing",
					"bucket_name", bucket.BucketName,
					"principal_type", input.PrincipalType,
					"principal_id", input.PrincipalID,
				)
				continue
			}
			slog.Info("object_list_policy_lookup_failed",
				"bucket_name", bucket.BucketName,
				"principal_type", input.PrincipalType,
				"principal_id", input.PrincipalID,
				"error", err,
			)
			return nil, fmt.Errorf("resolve authorization policy for list: %w", err)
		}
		if !policy.CanList {
			slog.Info("object_list_policy_denied",
				"bucket_name", bucket.BucketName,
				"principal_type", input.PrincipalType,
				"principal_id", input.PrincipalID,
			)
			continue
		}

		prefixes := intersectAllowedPrefixes(policy.ConnectionPrefixes, policy.PrincipalPrefixes)
		if len(prefixes) == 0 {
			slog.Info("object_list_no_intersecting_prefixes",
				"bucket_name", bucket.BucketName,
				"principal_type", input.PrincipalType,
				"principal_id", input.PrincipalID,
			)
			continue
		}
		slog.Info("object_list_prefixes_resolved",
			"bucket_name", bucket.BucketName,
			"prefix_count", len(prefixes),
		)

		for _, prefix := range prefixes {
			objects, err := s.lister.ListObjects(ctx, ObjectListRequest{
				BucketName: bucket.BucketName,
				Region:     bucket.Region,
				RoleARN:    bucket.RoleARN,
				ExternalID: bucket.ExternalID,
				Prefix:     prefix,
			})
			if err != nil {
				slog.Info("object_list_upstream_failed",
					"bucket_name", bucket.BucketName,
					"prefix", prefix,
					"error", err,
				)
				return nil, fmt.Errorf("list objects for prefix %q: %w", prefix, err)
			}
			slog.Info("object_list_prefix_completed",
				"bucket_name", bucket.BucketName,
				"prefix", prefix,
				"object_count", len(objects),
			)

			for _, object := range objects {
				if object.ObjectKey == "" || strings.HasSuffix(object.ObjectKey, "/") {
					continue
				}
				dedupeKey := bucket.BucketName + "\n" + object.ObjectKey
				if _, ok := seen[dedupeKey]; ok {
					continue
				}
				seen[dedupeKey] = struct{}{}
				entries = append(entries, ObjectListEntry{
					BucketName:   bucket.BucketName,
					ObjectKey:    object.ObjectKey,
					Size:         object.Size,
					ETag:         object.ETag,
					LastModified: object.LastModified,
				})
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].LastModified.Equal(entries[j].LastModified) {
			if entries[i].BucketName == entries[j].BucketName {
				return entries[i].ObjectKey < entries[j].ObjectKey
			}
			return entries[i].BucketName < entries[j].BucketName
		}
		return entries[i].LastModified.After(entries[j].LastModified)
	})

	slog.Info("object_list_completed",
		"project_id", input.ProjectID,
		"app_id", input.AppID,
		"result_count", len(entries),
	)
	return entries, nil
}

func intersectAllowedPrefixes(connectionPrefixes []string, principalPrefixes []string) []string {
	if len(connectionPrefixes) == 0 || len(principalPrefixes) == 0 {
		return nil
	}

	allowed := make(map[string]struct{}, len(connectionPrefixes))
	for _, prefix := range connectionPrefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		allowed[prefix] = struct{}{}
	}

	result := make([]string, 0, len(principalPrefixes))
	for _, prefix := range principalPrefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		if _, ok := allowed[prefix]; ok {
			result = append(result, prefix)
		}
	}

	return result
}
