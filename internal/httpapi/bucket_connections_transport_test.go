package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"s3-service/internal/auth"
)

type stubBucketConnectionService struct {
	buckets   []string
	err       error
	projectID string
	appID     string
}

func (s *stubBucketConnectionService) ListForScope(_ context.Context, projectID, appID string) ([]string, error) {
	s.projectID = projectID
	s.appID = appID
	if s.err != nil {
		return nil, s.err
	}
	return s.buckets, nil
}

type bucketConnectionsResponse struct {
	Data *struct {
		Buckets []string `json:"buckets"`
	} `json:"data"`
	Error any `json:"error"`
}

func TestListBucketConnectionsHandler(t *testing.T) {
	t.Run("returns unauthorized when claims are missing", func(t *testing.T) {
		svc := &stubBucketConnectionService{}
		h := listBucketConnectionsHandler(svc)

		req := httptest.NewRequest(http.MethodGet, "/v1/bucket-connections", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("forwards claim scope and returns buckets", func(t *testing.T) {
		svc := &stubBucketConnectionService{buckets: []string{"bucket-a", "bucket-b"}}
		h := listBucketConnectionsHandler(svc)

		claims := auth.Claims{Subject: "user-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleAdmin}
		req := httptest.NewRequest(http.MethodGet, "/v1/bucket-connections", nil)
		req = req.WithContext(context.WithValue(req.Context(), claimsContextKey{}, claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if svc.projectID != "project-1" || svc.appID != "app-1" {
			t.Fatalf("expected scope project-1/app-1, got %s/%s", svc.projectID, svc.appID)
		}

		var got bucketConnectionsResponse
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.Data == nil {
			t.Fatal("expected data envelope")
		}
		if len(got.Data.Buckets) != 2 || got.Data.Buckets[0] != "bucket-a" || got.Data.Buckets[1] != "bucket-b" {
			t.Fatalf("unexpected buckets: %+v", got.Data.Buckets)
		}
	})

	t.Run("returns internal server error when service fails", func(t *testing.T) {
		svc := &stubBucketConnectionService{err: errors.New("db down")}
		h := listBucketConnectionsHandler(svc)

		claims := auth.Claims{Subject: "user-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleAdmin}
		req := httptest.NewRequest(http.MethodGet, "/v1/bucket-connections", nil)
		req = req.WithContext(context.WithValue(req.Context(), claimsContextKey{}, claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", rec.Code)
		}
	})
}
