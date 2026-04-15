package router_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"s3-service/internal/auth"
	"s3-service/internal/database"
	httpmiddleware "s3-service/internal/httpapi/middleware"
	"s3-service/internal/httpapi/router"
	"s3-service/internal/service"
)

type matrixBucketService struct {
	buckets []database.BucketConnection
}

func (s *matrixBucketService) ListForScope(_ context.Context, _, _ string) ([]database.BucketConnection, error) {
	result := make([]database.BucketConnection, 0, len(s.buckets))
	result = append(result, s.buckets...)
	return result, nil
}

func (s *matrixBucketService) CreateForScope(_ context.Context, _, _ string, bucketName string, region string, roleARN string, externalID *string, allowedPrefixes []string) error {
	s.buckets = append(s.buckets, database.BucketConnection{
		BucketName:      bucketName,
		Region:          region,
		RoleARN:         roleARN,
		ExternalID:      externalID,
		AllowedPrefixes: allowedPrefixes,
	})
	return nil
}

func (s *matrixBucketService) UpsertAccessPolicyForScope(_ context.Context, _, _, _, _, _, _ string, _, _, _, _ bool, _ []string) error {
	return nil
}

type matrixAuthorizationService struct{}

func (s *matrixAuthorizationService) Authorize(_ context.Context, _ auth.AuthorizationRequest) auth.Decision {
	return auth.Decision{Allowed: true}
}

type matrixObjectUploadService struct{}

func (s *matrixObjectUploadService) UploadObject(_ context.Context, input service.ObjectUploadInput) (service.ObjectUploadResult, error) {
	return service.ObjectUploadResult{ETag: "\"matrix-etag\"", Size: int64(len(input.Body))}, nil
}

type matrixObjectDeleteService struct {
	deleted []service.ObjectDeleteInput
}

func (s *matrixObjectDeleteService) DeleteObject(_ context.Context, input service.ObjectDeleteInput) (service.ObjectDeleteResult, error) {
	s.deleted = append(s.deleted, input)
	return service.ObjectDeleteResult{Deleted: true}, nil
}

type matrixObjectPresignService struct{}

func (s *matrixObjectPresignService) PresignObject(_ context.Context, input service.ObjectPresignInput) (service.ObjectPresignResult, error) {
	return service.ObjectPresignResult{Method: input.Method, URL: "https://example.test/presigned", ExpiresIn: input.ExpiresIn}, nil
}

type matrixObjectListService struct{}

func (s *matrixObjectListService) ListImages(_ context.Context, _ service.ObjectListInput) ([]service.ObjectListEntry, error) {
	return []service.ObjectListEntry{
		{BucketName: "bucket-a", ObjectKey: "images/cat.jpg", Size: 12345, ETag: "\"matrix-etag\"", LastModified: time.Date(2026, time.April, 12, 3, 14, 15, 0, time.UTC)},
	}, nil
}

type matrixObjectReadService struct{}

func (s *matrixObjectReadService) ReadObject(_ context.Context, _ service.ObjectReadInput) (service.ObjectReadResult, error) {
	return service.ObjectReadResult{
		Body:          io.NopCloser(strings.NewReader("jpeg-bytes")),
		ContentType:   "image/jpeg",
		ContentLength: int64(len("jpeg-bytes")),
		ETag:          "\"matrix-etag\"",
	}, nil
}

func matrixAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := auth.Claims{
			Subject:       "user-1",
			AppID:         "app-1",
			ProjectID:     "project-1",
			Role:          auth.RoleAdmin,
			PrincipalType: auth.PrincipalTypeUser,
		}
		next.ServeHTTP(w, r.WithContext(httpmiddleware.ContextWithClaims(r.Context(), claims)))
	})
}

func newMatrixRouter(deleteSvc *matrixObjectDeleteService, bucketSvc *matrixBucketService) http.Handler {
	return router.NewRouter(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		matrixAuthMiddleware,
		bucketSvc,
		&matrixAuthorizationService{},
		&matrixObjectUploadService{},
		deleteSvc,
		&matrixObjectPresignService{},
		&matrixObjectListService{},
		&matrixObjectReadService{},
		nil,
	)
}

func TestAPIMatrix_AllActiveEndpoints_HappyPath(t *testing.T) {
	deleteSvc := &matrixObjectDeleteService{}
	bucketSvc := &matrixBucketService{buckets: []database.BucketConnection{{BucketName: "bucket-a", Region: "us-east-1", RoleARN: "arn:aws:iam::123456789012:role/s3-runtime-role", AllowedPrefixes: []string{"images/"}}}}
	r := newMatrixRouter(deleteSvc, bucketSvc)

	imageID := base64.RawURLEncoding.EncodeToString([]byte("bucket-a:images/cat.jpg"))

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{name: "health", method: http.MethodGet, path: "/health", wantStatus: http.StatusOK},
		{name: "auth-check", method: http.MethodGet, path: "/v1/auth-check", wantStatus: http.StatusOK},
		{name: "create bucket connection", method: http.MethodPost, path: "/v1/bucket-connections", body: `{"bucket_name":"bucket-b","region":"us-west-2","role_arn":"arn:aws:iam::123456789012:role/s3-runtime-role","allowed_prefixes":["images/"]}`, wantStatus: http.StatusCreated},
		{name: "upsert access policy", method: http.MethodPost, path: "/v1/access-policies", body: `{"bucket_name":"bucket-a","principal_type":"service","principal_id":"auth0|svc-1","role":"admin","can_read":true,"can_write":true,"can_delete":false,"can_list":true,"prefix_allowlist":["images/"]}`, wantStatus: http.StatusOK},
		{name: "list bucket connections", method: http.MethodGet, path: "/v1/bucket-connections", wantStatus: http.StatusOK},
		{name: "upload object", method: http.MethodPost, path: "/v1/objects/upload", body: `{"bucket_name":"bucket-a","object_key":"images/cat.jpg","content_type":"image/jpeg","content_b64":"aGVsbG8="}`, wantStatus: http.StatusCreated},
		{name: "delete object", method: http.MethodDelete, path: "/v1/objects", body: `{"bucket_name":"bucket-a","object_key":"images/cat.jpg"}`, wantStatus: http.StatusOK},
		{name: "presign upload", method: http.MethodPost, path: "/v1/objects/presign-upload", body: `{"bucket_name":"bucket-a","object_key":"images/cat.jpg","content_type":"image/jpeg","expires_in_seconds":60}`, wantStatus: http.StatusOK},
		{name: "presign download", method: http.MethodPost, path: "/v1/objects/presign-download", body: `{"bucket_name":"bucket-a","object_key":"images/cat.jpg","expires_in_seconds":120}`, wantStatus: http.StatusOK},
		{name: "list images discovery", method: http.MethodGet, path: "/v1/images", wantStatus: http.StatusOK},
		{name: "list images resolve", method: http.MethodGet, path: "/v1/images?ids=" + imageID, wantStatus: http.StatusOK},
		{name: "get image bytes", method: http.MethodGet, path: "/v1/images/" + imageID, wantStatus: http.StatusOK},
		{name: "delete image", method: http.MethodDelete, path: "/v1/images/" + imageID, wantStatus: http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.path, body)
			req.RemoteAddr = "203.0.113.10:43210"
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d body=%s", tc.wantStatus, rec.Code, rec.Body.String())
			}

			if strings.HasPrefix(tc.path, "/v1/") && tc.path != "/v1/images/"+imageID {
				var envelope struct {
					Data  any `json:"data"`
					Error any `json:"error"`
				}
				if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
					t.Fatalf("expected json envelope response, decode failed: %v", err)
				}
				if envelope.Data == nil {
					t.Fatalf("expected data envelope for %s", tc.path)
				}
			}
		})
	}
}

func TestAPIMatrix_CreateRemoveFlow_ImageLifecycle(t *testing.T) {
	deleteSvc := &matrixObjectDeleteService{}
	bucketSvc := &matrixBucketService{}
	r := newMatrixRouter(deleteSvc, bucketSvc)

	createBucketReq := httptest.NewRequest(http.MethodPost, "/v1/bucket-connections", strings.NewReader(`{"bucket_name":"bucket-a","region":"us-east-1","role_arn":"arn:aws:iam::123456789012:role/s3-runtime-role","allowed_prefixes":["images/"]}`))
	createBucketReq.RemoteAddr = "203.0.113.10:43210"
	createBucketReq.Header.Set("Content-Type", "application/json")
	createBucketRec := httptest.NewRecorder()
	r.ServeHTTP(createBucketRec, createBucketReq)
	if createBucketRec.Code != http.StatusCreated {
		t.Fatalf("expected create bucket status 201, got %d body=%s", createBucketRec.Code, createBucketRec.Body.String())
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/v1/objects/upload", strings.NewReader(`{"bucket_name":"bucket-a","object_key":"images/cat.jpg","content_type":"image/jpeg","content_b64":"aGVsbG8="}`))
	uploadReq.RemoteAddr = "203.0.113.10:43210"
	uploadReq.Header.Set("Content-Type", "application/json")
	uploadRec := httptest.NewRecorder()
	r.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("expected upload status 201, got %d body=%s", uploadRec.Code, uploadRec.Body.String())
	}

	imageID := base64.RawURLEncoding.EncodeToString([]byte("bucket-a:images/cat.jpg"))
	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/images/"+imageID, nil)
	deleteReq.RemoteAddr = "203.0.113.10:43210"
	deleteRec := httptest.NewRecorder()
	r.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete image status 200, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}

	if len(deleteSvc.deleted) == 0 {
		t.Fatal("expected delete service to be called")
	}
	lastDelete := deleteSvc.deleted[len(deleteSvc.deleted)-1]
	if lastDelete.BucketName != "bucket-a" || lastDelete.ObjectKey != "images/cat.jpg" {
		t.Fatalf("unexpected delete input: %+v", lastDelete)
	}
}
