package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"s3-service/internal/auth"
	httpmiddleware "s3-service/internal/httpapi/middleware"
	"s3-service/internal/s3"
	"s3-service/internal/service"
)

type stubImageReadService struct {
	result service.ObjectReadResult
	err    error
	input  service.ObjectReadInput
}

type stubImageListService struct {
	items []service.ObjectListEntry
	err   error
	input service.ObjectListInput
}

func (s *stubImageReadService) ReadObject(_ context.Context, input service.ObjectReadInput) (service.ObjectReadResult, error) {
	s.input = input
	if s.err != nil {
		return service.ObjectReadResult{}, s.err
	}
	return s.result, nil
}

func (s *stubImageListService) ListImages(_ context.Context, input service.ObjectListInput) ([]service.ObjectListEntry, error) {
	s.input = input
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}

func TestListImagesHandler(t *testing.T) {
	claims := auth.Claims{Subject: "user-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleProjectClient, PrincipalType: auth.PrincipalTypeUser}

	t.Run("returns unauthorized when claims are missing", func(t *testing.T) {
		h := ListImagesHandler(nil)

		req := httptest.NewRequest(http.MethodGet, "/v1/images", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("returns not implemented when list service is missing", func(t *testing.T) {
		h := ListImagesHandler(nil)

		req := httptest.NewRequest(http.MethodGet, "/v1/images", nil)
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("expected status 501, got %d", rec.Code)
		}
	})

	t.Run("resolves provided ids", func(t *testing.T) {
		idA := encodeImageID("bucket-a", "images/a.jpg")
		idB := encodeImageID("bucket-b", "images/b.jpg")
		h := ListImagesHandler(nil)

		req := httptest.NewRequest(http.MethodGet, "/v1/images?ids="+idA+","+idB+"&ids="+idA, nil)
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var got struct {
			Data *struct {
				Images []struct {
					ID         string `json:"id"`
					BucketName string `json:"bucket_name"`
					ObjectKey  string `json:"object_key"`
					URL        string `json:"url"`
				} `json:"images"`
			} `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.Data == nil || len(got.Data.Images) != 2 {
			t.Fatalf("expected two resolved images, got %+v", got.Data)
		}
		if got.Data.Images[0].ID != idA || got.Data.Images[0].BucketName != "bucket-a" || got.Data.Images[0].ObjectKey != "images/a.jpg" || got.Data.Images[0].URL != "/v1/images/"+idA {
			t.Fatalf("unexpected first item: %+v", got.Data.Images[0])
		}
		if got.Data.Images[1].ID != idB || got.Data.Images[1].BucketName != "bucket-b" || got.Data.Images[1].ObjectKey != "images/b.jpg" || got.Data.Images[1].URL != "/v1/images/"+idB {
			t.Fatalf("unexpected second item: %+v", got.Data.Images[1])
		}
	})

	t.Run("returns bad request when any provided id is invalid", func(t *testing.T) {
		h := ListImagesHandler(nil)

		req := httptest.NewRequest(http.MethodGet, "/v1/images?ids=not-valid", nil)
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("returns image list with backend urls", func(t *testing.T) {
		now := time.Date(2026, time.April, 11, 12, 0, 0, 0, time.UTC)
		listSvc := &stubImageListService{items: []service.ObjectListEntry{{BucketName: "bucket-a", ObjectKey: "images/a.jpg", Size: 100, ETag: "etag-a", LastModified: now}}}
		h := ListImagesHandler(listSvc)

		req := httptest.NewRequest(http.MethodGet, "/v1/images", nil)
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if listSvc.input.ProjectID != "project-1" || listSvc.input.AppID != "app-1" || listSvc.input.PrincipalType != "user" || listSvc.input.PrincipalID != "user-1" {
			t.Fatalf("unexpected list scope input: %+v", listSvc.input)
		}

		var got struct {
			Data *struct {
				Images []struct {
					ID           string `json:"id"`
					BucketName   string `json:"bucket_name"`
					ObjectKey    string `json:"object_key"`
					SizeBytes    int64  `json:"size_bytes"`
					ETag         string `json:"etag"`
					LastModified string `json:"last_modified"`
					URL          string `json:"url"`
				} `json:"images"`
			} `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.Data == nil || len(got.Data.Images) != 1 {
			t.Fatalf("expected one image in response, got %+v", got.Data)
		}
		if got.Data.Images[0].BucketName != "bucket-a" || got.Data.Images[0].ObjectKey != "images/a.jpg" || got.Data.Images[0].SizeBytes != 100 || got.Data.Images[0].ETag != "etag-a" {
			t.Fatalf("unexpected image payload: %+v", got.Data.Images[0])
		}
		expectedID := encodeImageID("bucket-a", "images/a.jpg")
		if got.Data.Images[0].ID != expectedID {
			t.Fatalf("expected id %q, got %q", expectedID, got.Data.Images[0].ID)
		}
		if got.Data.Images[0].URL != "/v1/images/"+expectedID {
			t.Fatalf("unexpected url: %q", got.Data.Images[0].URL)
		}
		if got.Data.Images[0].LastModified != now.Format(time.RFC3339) {
			t.Fatalf("unexpected last_modified: %q", got.Data.Images[0].LastModified)
		}
	})

	t.Run("returns upstream failure when list service errors", func(t *testing.T) {
		h := ListImagesHandler(&stubImageListService{err: errors.New("s3 timeout")})

		req := httptest.NewRequest(http.MethodGet, "/v1/images", nil)
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected status 502, got %d", rec.Code)
		}
	})

}

func TestGetImageHandler(t *testing.T) {
	claims := auth.Claims{Subject: "user-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleProjectClient, PrincipalType: auth.PrincipalTypeUser}

	t.Run("returns unauthorized when claims are missing", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{}
		h := GetImageHandler(authz, nil)

		req := requestWithImageID(http.MethodGet, "/v1/images/ignored", "ignored")
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("returns bad request for invalid image id", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{}
		h := GetImageHandler(authz, nil)

		req := requestWithImageID(http.MethodGet, "/v1/images/not-base64", "not-base64")
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("returns forbidden when authorization denies", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: false, Reason: auth.DecisionReasonPrefixScope}}
		h := GetImageHandler(authz, nil)

		id := encodeImageID("bucket-a", "uploads/a.jpg")
		req := requestWithImageID(http.MethodGet, "/v1/images/"+id, id)
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", rec.Code)
		}
		if authz.request.Action != auth.ActionRead || authz.request.BucketName != "bucket-a" || authz.request.ObjectKey != "uploads/a.jpg" {
			t.Fatalf("unexpected authz request: %+v", authz.request)
		}
	})

	t.Run("returns not implemented when read service is not configured", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: true}}
		h := GetImageHandler(authz, nil)

		id := encodeImageID("bucket-a", "uploads/a.jpg")
		req := requestWithImageID(http.MethodGet, "/v1/images/"+id, id)
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("expected status 501, got %d", rec.Code)
		}
	})

	t.Run("returns not found when object is missing", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: true}}
		h := GetImageHandler(authz, &stubImageReadService{err: s3.ErrObjectNotFound})

		id := encodeImageID("bucket-a", "uploads/a.jpg")
		req := requestWithImageID(http.MethodGet, "/v1/images/"+id, id)
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
		var got apiErrorEnvelope
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.Error == nil || got.Error.Code != "not_found" {
			t.Fatalf("expected not_found error, got %+v", got.Error)
		}
	})

	t.Run("streams object when read succeeds", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: true}}
		readSvc := &stubImageReadService{result: service.ObjectReadResult{Body: io.NopCloser(strings.NewReader("payload")), ContentType: "image/jpeg", ContentLength: 7, ETag: "\"etag-1\""}}
		h := GetImageHandler(authz, readSvc)

		id := encodeImageID("bucket-a", "uploads/a.jpg")
		req := requestWithImageID(http.MethodGet, "/v1/images/"+id, id)
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if readSvc.input.ProjectID != "project-1" || readSvc.input.AppID != "app-1" {
			t.Fatalf("unexpected read scope: %+v", readSvc.input)
		}
		if readSvc.input.BucketName != "bucket-a" || readSvc.input.ObjectKey != "uploads/a.jpg" {
			t.Fatalf("unexpected read input: %+v", readSvc.input)
		}
		if got := rec.Header().Get("Content-Type"); got != "image/jpeg" {
			t.Fatalf("expected content type image/jpeg, got %q", got)
		}
		if got := rec.Header().Get("ETag"); got != "\"etag-1\"" {
			t.Fatalf("expected etag \"etag-1\", got %q", got)
		}
		if rec.Body.String() != "payload" {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("returns bucket not found when scope has no matching connection", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: true}}
		h := GetImageHandler(authz, &stubImageReadService{err: service.ErrBucketConnectionNotFound})

		id := encodeImageID("bucket-a", "uploads/a.jpg")
		req := requestWithImageID(http.MethodGet, "/v1/images/"+id, id)
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
	})

	t.Run("returns upstream failure for unexpected errors", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: true}}
		h := GetImageHandler(authz, &stubImageReadService{err: errors.New("s3 timeout")})

		id := encodeImageID("bucket-a", "uploads/a.jpg")
		req := requestWithImageID(http.MethodGet, "/v1/images/"+id, id)
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected status 502, got %d", rec.Code)
		}
	})
}

func requestWithImageID(method string, path string, id string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
