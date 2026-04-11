package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"s3-service/internal/database"
)

type stubObjectListBucketRepo struct {
	buckets   []database.BucketConnection
	err       error
	projectID string
	appID     string
}

func (s *stubObjectListBucketRepo) ListActiveBucketsForConnectionScope(_ context.Context, projectID string, appID string) ([]database.BucketConnection, error) {
	s.projectID = projectID
	s.appID = appID
	if s.err != nil {
		return nil, s.err
	}
	return s.buckets, nil
}

type stubObjectListPolicyRepo struct {
	policyByBucket map[string]database.EffectiveAuthorizationPolicy
	errByBucket    map[string]error
	lookups        []database.AuthorizationPolicyLookup
}

func (s *stubObjectListPolicyRepo) GetEffectiveAuthorizationPolicy(_ context.Context, lookup database.AuthorizationPolicyLookup) (database.EffectiveAuthorizationPolicy, error) {
	s.lookups = append(s.lookups, lookup)
	if err := s.errByBucket[lookup.BucketName]; err != nil {
		return database.EffectiveAuthorizationPolicy{}, err
	}
	policy, ok := s.policyByBucket[lookup.BucketName]
	if !ok {
		return database.EffectiveAuthorizationPolicy{}, database.ErrPolicyNotFound
	}
	return policy, nil
}

type stubObjectLister struct {
	objectsByPrefix map[string][]ObjectListObject
	errByPrefix     map[string]error
	calls           []ObjectListRequest
}

func (s *stubObjectLister) ListObjects(_ context.Context, input ObjectListRequest) ([]ObjectListObject, error) {
	s.calls = append(s.calls, input)
	if err := s.errByPrefix[input.Prefix]; err != nil {
		return nil, err
	}
	return s.objectsByPrefix[input.Prefix], nil
}

func TestObjectListService_ListImages(t *testing.T) {
	t.Run("lists across allowed prefixes and returns backend-friendly metadata", func(t *testing.T) {
		t1 := time.Date(2026, time.April, 11, 12, 0, 0, 0, time.UTC)
		t2 := t1.Add(5 * time.Minute)

		bucketRepo := &stubObjectListBucketRepo{buckets: []database.BucketConnection{{BucketName: "bucket-a", Region: "us-east-1", RoleARN: "arn:aws:iam::123:role/s3"}}}
		policyRepo := &stubObjectListPolicyRepo{policyByBucket: map[string]database.EffectiveAuthorizationPolicy{
			"bucket-a": {
				CanList:            true,
				ConnectionPrefixes: []string{"images/", "uploads/"},
				PrincipalPrefixes:  []string{"images/"},
			},
		}}
		lister := &stubObjectLister{objectsByPrefix: map[string][]ObjectListObject{
			"images/": {
				{ObjectKey: "images/a.jpg", Size: 100, ETag: "etag-a", LastModified: t1},
				{ObjectKey: "images/b.jpg", Size: 120, ETag: "etag-b", LastModified: t2},
			},
		}}

		svc := NewObjectListService(bucketRepo, policyRepo, lister)
		items, err := svc.ListImages(context.Background(), ObjectListInput{
			ProjectID:     "project-1",
			AppID:         "app-1",
			PrincipalType: "user",
			PrincipalID:   "user-1",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if bucketRepo.projectID != "project-1" || bucketRepo.appID != "app-1" {
			t.Fatalf("unexpected scope lookup: %s/%s", bucketRepo.projectID, bucketRepo.appID)
		}
		if len(lister.calls) != 1 || lister.calls[0].Prefix != "images/" {
			t.Fatalf("expected one lister call for images/ prefix, got %+v", lister.calls)
		}
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
		if items[0].ObjectKey != "images/b.jpg" || items[1].ObjectKey != "images/a.jpg" {
			t.Fatalf("expected newest-first ordering, got %+v", items)
		}
	})

	t.Run("filters buckets without policy or list permission", func(t *testing.T) {
		bucketRepo := &stubObjectListBucketRepo{buckets: []database.BucketConnection{{BucketName: "bucket-a"}, {BucketName: "bucket-b"}}}
		policyRepo := &stubObjectListPolicyRepo{
			policyByBucket: map[string]database.EffectiveAuthorizationPolicy{"bucket-a": {CanList: false}},
		}
		lister := &stubObjectLister{}

		svc := NewObjectListService(bucketRepo, policyRepo, lister)
		items, err := svc.ListImages(context.Background(), ObjectListInput{
			ProjectID:     "project-1",
			AppID:         "app-1",
			PrincipalType: "user",
			PrincipalID:   "user-1",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(items) != 0 {
			t.Fatalf("expected no items, got %+v", items)
		}
		if len(lister.calls) != 0 {
			t.Fatalf("expected no lister calls, got %+v", lister.calls)
		}
	})

	t.Run("returns validation error for missing required fields", func(t *testing.T) {
		svc := NewObjectListService(&stubObjectListBucketRepo{}, &stubObjectListPolicyRepo{}, &stubObjectLister{})

		_, err := svc.ListImages(context.Background(), ObjectListInput{ProjectID: "project-1", AppID: "app-1"})
		if err == nil {
			t.Fatal("expected validation error")
		}
		if !errors.Is(err, ErrInvalidObjectListInput) {
			t.Fatalf("expected ErrInvalidObjectListInput, got %v", err)
		}
	})

	t.Run("returns list error when lister fails", func(t *testing.T) {
		bucketRepo := &stubObjectListBucketRepo{buckets: []database.BucketConnection{{BucketName: "bucket-a", Region: "us-east-1", RoleARN: "arn:aws:iam::123:role/s3"}}}
		policyRepo := &stubObjectListPolicyRepo{policyByBucket: map[string]database.EffectiveAuthorizationPolicy{"bucket-a": {CanList: true, ConnectionPrefixes: []string{"images/"}, PrincipalPrefixes: []string{"images/"}}}}
		lister := &stubObjectLister{errByPrefix: map[string]error{"images/": errors.New("s3 timeout")}}

		svc := NewObjectListService(bucketRepo, policyRepo, lister)
		_, err := svc.ListImages(context.Background(), ObjectListInput{
			ProjectID:     "project-1",
			AppID:         "app-1",
			PrincipalType: "user",
			PrincipalID:   "user-1",
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
