package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"s3-service/internal/auth"
	"s3-service/internal/database"
	httpmiddleware "s3-service/internal/httpapi/middleware"
	"s3-service/internal/s3"
	"s3-service/internal/service"
)

type stubBucketConnectionService struct {
	buckets         []database.BucketConnection
	err             error
	projectID       string
	appID           string
	bucketName      string
	region          string
	roleARN         string
	externalID      *string
	allowedPrefixes []string
}

func (s *stubBucketConnectionService) ListForScope(_ context.Context, projectID, appID string) ([]database.BucketConnection, error) {
	s.projectID = projectID
	s.appID = appID
	if s.err != nil {
		return nil, s.err
	}
	return s.buckets, nil
}

func (s *stubBucketConnectionService) CreateForScope(_ context.Context, projectID string, appID string, bucketName string, region string, roleARN string, externalID *string, allowedPrefixes []string) error {
	s.projectID = projectID
	s.appID = appID
	s.bucketName = bucketName
	s.region = region
	s.roleARN = roleARN
	s.externalID = externalID
	s.allowedPrefixes = allowedPrefixes
	if s.err != nil {
		return s.err
	}
	return nil
}

type bucketConnectionsResponse struct {
	Data *struct {
		Buckets []struct {
			BucketName      string   `json:"bucket_name"`
			Region          string   `json:"region"`
			RoleARN         string   `json:"role_arn"`
			ExternalID      *string  `json:"external_id"`
			AllowedPrefixes []string `json:"allowed_prefixes"`
		} `json:"buckets"`
	} `json:"data"`
	Error *apiErrorBody `json:"error"`
}

func TestListBucketConnectionsHandler(t *testing.T) {
	t.Run("returns unauthorized when claims are missing", func(t *testing.T) {
		svc := &stubBucketConnectionService{}
		h := ListBucketConnectionsHandler(svc)

		req := httptest.NewRequest(http.MethodGet, "/v1/bucket-connections", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("forwards claim scope and returns buckets", func(t *testing.T) {
		svc := &stubBucketConnectionService{buckets: []database.BucketConnection{{BucketName: "bucket-a", Region: "us-east-1", RoleARN: "arn:aws:iam::123456789012:role/s3-a"}, {BucketName: "bucket-b", Region: "us-west-2", RoleARN: "arn:aws:iam::123456789012:role/s3-b"}}}
		h := ListBucketConnectionsHandler(svc)

		claims := auth.Claims{Subject: "user-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleAdmin}
		req := httptest.NewRequest(http.MethodGet, "/v1/bucket-connections", nil)
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
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
		if len(got.Data.Buckets) != 2 || got.Data.Buckets[0].BucketName != "bucket-a" || got.Data.Buckets[1].BucketName != "bucket-b" {
			t.Fatalf("unexpected buckets: %+v", got.Data.Buckets)
		}
		if got.Data.Buckets[0].Region != "us-east-1" || got.Data.Buckets[1].Region != "us-west-2" {
			t.Fatalf("unexpected buckets: %+v", got.Data.Buckets)
		}
	})

	t.Run("returns internal server error when service fails", func(t *testing.T) {
		svc := &stubBucketConnectionService{err: errors.New("db down")}
		h := ListBucketConnectionsHandler(svc)

		claims := auth.Claims{Subject: "user-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleAdmin}
		req := httptest.NewRequest(http.MethodGet, "/v1/bucket-connections", nil)
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", rec.Code)
		}
	})
}

func TestCreateBucketConnectionHandler(t *testing.T) {
	t.Run("returns unauthorized when claims are missing", func(t *testing.T) {
		svc := &stubBucketConnectionService{}
		h := CreateBucketConnectionHandler(svc)

		req := httptest.NewRequest(http.MethodPost, "/v1/bucket-connections", strings.NewReader(`{"bucket_name":"bucket-a","region":"us-east-1","role_arn":"arn:aws:iam::123456789012:role/s3"}`))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("returns bad request for invalid json", func(t *testing.T) {
		svc := &stubBucketConnectionService{}
		h := CreateBucketConnectionHandler(svc)

		claims := auth.Claims{Subject: "user-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleAdmin}
		req := httptest.NewRequest(http.MethodPost, "/v1/bucket-connections", strings.NewReader("{"))
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("returns bad request when service rejects missing required fields", func(t *testing.T) {
		svc := &stubBucketConnectionService{err: service.ErrInvalidBucketConnectionInput}
		h := CreateBucketConnectionHandler(svc)

		claims := auth.Claims{Subject: "user-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleAdmin}
		body := `{"bucket_name":"","region":"us-east-1","role_arn":""}`
		req := httptest.NewRequest(http.MethodPost, "/v1/bucket-connections", strings.NewReader(body))
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("creates connection for claim scope", func(t *testing.T) {
		svc := &stubBucketConnectionService{}
		h := CreateBucketConnectionHandler(svc)

		claims := auth.Claims{Subject: "user-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleAdmin}
		body := `{"bucket_name":"bucket-a","region":"us-east-1","role_arn":"arn:aws:iam::123456789012:role/s3","external_id":"ext-1","allowed_prefixes":["uploads/","avatars/"]}`
		req := httptest.NewRequest(http.MethodPost, "/v1/bucket-connections", strings.NewReader(body))
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", rec.Code)
		}
		if svc.projectID != "project-1" || svc.appID != "app-1" {
			t.Fatalf("expected scope project-1/app-1, got %s/%s", svc.projectID, svc.appID)
		}
		if svc.bucketName != "bucket-a" || svc.region != "us-east-1" || svc.roleARN == "" {
			t.Fatalf("unexpected create args: bucket=%s region=%s role=%s", svc.bucketName, svc.region, svc.roleARN)
		}
		if svc.externalID == nil || *svc.externalID != "ext-1" {
			t.Fatalf("expected external id ext-1, got %+v", svc.externalID)
		}
		if len(svc.allowedPrefixes) != 2 || svc.allowedPrefixes[0] != "uploads/" || svc.allowedPrefixes[1] != "avatars/" {
			t.Fatalf("unexpected allowed prefixes: %+v", svc.allowedPrefixes)
		}
	})

	t.Run("returns conflict when connection already exists", func(t *testing.T) {
		svc := &stubBucketConnectionService{err: database.ErrBucketConnectionAlreadyExists}
		h := CreateBucketConnectionHandler(svc)

		claims := auth.Claims{Subject: "user-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleAdmin}
		body := `{"bucket_name":"bucket-a","region":"us-east-1","role_arn":"arn:aws:iam::123456789012:role/s3"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/bucket-connections", strings.NewReader(body))
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d", rec.Code)
		}
	})

	t.Run("returns bad request when bucket security baseline check fails", func(t *testing.T) {
		svc := &stubBucketConnectionService{err: &s3.BucketSecurityBaselineError{BucketName: "bucket-a", Reasons: []string{"bucket policy allows public access"}}}
		h := CreateBucketConnectionHandler(svc)

		claims := auth.Claims{Subject: "user-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleAdmin}
		body := `{"bucket_name":"bucket-a","region":"us-east-1","role_arn":"arn:aws:iam::123456789012:role/s3"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/bucket-connections", strings.NewReader(body))
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
		var got apiErrorEnvelope
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.Error == nil || got.Error.Code != "bucket_security_baseline_failed" {
			t.Fatalf("expected bucket_security_baseline_failed, got %+v", got.Error)
		}
	})
}
