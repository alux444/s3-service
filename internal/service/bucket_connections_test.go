package service

import (
	"context"
	"errors"
	"testing"
)

type stubBucketConnectionsRepo struct {
	buckets   []string
	err       error
	projectID string
	appID     string
}

func (s *stubBucketConnectionsRepo) ListActiveBucketsForConnectionScope(_ context.Context, projectID string, appID string) ([]string, error) {
	s.projectID = projectID
	s.appID = appID
	if s.err != nil {
		return nil, s.err
	}
	return s.buckets, nil
}

func TestBucketConnectionsService_ListForScope(t *testing.T) {
	t.Run("forwards scope and returns buckets", func(t *testing.T) {
		repo := &stubBucketConnectionsRepo{buckets: []string{"bucket-a", "bucket-b"}}
		svc := NewBucketConnectionsService(repo)

		got, err := svc.ListForScope(context.Background(), "project-1", "app-1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if repo.projectID != "project-1" || repo.appID != "app-1" {
			t.Fatalf("expected scope project-1/app-1, got %s/%s", repo.projectID, repo.appID)
		}
		if len(got) != 2 || got[0] != "bucket-a" || got[1] != "bucket-b" {
			t.Fatalf("unexpected buckets: %+v", got)
		}
	})

	t.Run("returns repository error", func(t *testing.T) {
		repo := &stubBucketConnectionsRepo{err: errors.New("repo failed")}
		svc := NewBucketConnectionsService(repo)

		_, err := svc.ListForScope(context.Background(), "project-1", "app-1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
