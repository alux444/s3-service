package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func (s *stubImageReadService) ReadObject(_ context.Context, input service.ObjectReadInput) (service.ObjectReadResult, error) {
	s.input = input
	if s.err != nil {
		return service.ObjectReadResult{}, s.err
	}
	return s.result, nil
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

func encodeImageID(bucketName string, objectKey string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(bucketName + ":" + objectKey))
}
