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
	httpmiddleware "s3-service/internal/httpapi/middleware"
	"s3-service/internal/service"
)

type stubObjectAuthorizationService struct {
	decision auth.Decision
	request  auth.AuthorizationRequest
}

func (s *stubObjectAuthorizationService) Authorize(_ context.Context, request auth.AuthorizationRequest) auth.Decision {
	s.request = request
	return s.decision
}

type stubObjectUploadService struct {
	result service.ObjectUploadResult
	err    error
	input  service.ObjectUploadInput
}

func (s *stubObjectUploadService) UploadObject(_ context.Context, input service.ObjectUploadInput) (service.ObjectUploadResult, error) {
	s.input = input
	if s.err != nil {
		return service.ObjectUploadResult{}, s.err
	}
	return s.result, nil
}

type stubObjectDeleteService struct {
	result service.ObjectDeleteResult
	err    error
	input  service.ObjectDeleteInput
}

func (s *stubObjectDeleteService) DeleteObject(_ context.Context, input service.ObjectDeleteInput) (service.ObjectDeleteResult, error) {
	s.input = input
	if s.err != nil {
		return service.ObjectDeleteResult{}, s.err
	}
	return s.result, nil
}

func TestUploadObjectHandler(t *testing.T) {
	claims := auth.Claims{Subject: "user-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleProjectClient, PrincipalType: auth.PrincipalTypeUser}

	t.Run("returns forbidden when authorization denies", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: false, Reason: auth.DecisionReasonPrefixScope}}
		h := UploadObjectHandler(authz, nil)

		req := httptest.NewRequest(http.MethodPost, "/v1/objects/upload", strings.NewReader(`{"bucket_name":"bucket-a","object_key":"uploads/a.jpg","content_type":"image/jpeg","content_b64":"cGF5bG9hZA=="}`))
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", rec.Code)
		}
		if authz.request.Action != auth.ActionWrite || authz.request.BucketName != "bucket-a" {
			t.Fatalf("unexpected authz request: %+v", authz.request)
		}
	})

	t.Run("returns not implemented when upload service is not configured", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: true}}
		h := UploadObjectHandler(authz, nil)

		req := httptest.NewRequest(http.MethodPost, "/v1/objects/upload", strings.NewReader(`{"bucket_name":"bucket-a","object_key":"uploads/a.jpg","content_type":"image/jpeg","content_b64":"cGF5bG9hZA=="}`))
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("expected status 501, got %d", rec.Code)
		}
		var got apiErrorEnvelope
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.Error == nil || got.Error.Code != "not_implemented" {
			t.Fatalf("expected not_implemented error, got %+v", got.Error)
		}
	})

	t.Run("returns created when upload succeeds", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: true}}
		uploadSvc := &stubObjectUploadService{result: service.ObjectUploadResult{ETag: "etag-1", Size: 7}}
		h := UploadObjectHandler(authz, uploadSvc)

		req := httptest.NewRequest(http.MethodPost, "/v1/objects/upload", strings.NewReader(`{"bucket_name":"bucket-a","object_key":"uploads/a.jpg","content_type":"image/jpeg","content_b64":"cGF5bG9hZA=="}`))
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", rec.Code)
		}
		if uploadSvc.input.ProjectID != "project-1" || uploadSvc.input.AppID != "app-1" {
			t.Fatalf("unexpected upload scope: project=%s app=%s", uploadSvc.input.ProjectID, uploadSvc.input.AppID)
		}
		if uploadSvc.input.ContentType != "image/jpeg" || string(uploadSvc.input.Body) != "payload" {
			t.Fatalf("unexpected upload input: %+v", uploadSvc.input)
		}
	})

	t.Run("returns bad request when base64 payload is invalid", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: true}}
		h := UploadObjectHandler(authz, &stubObjectUploadService{})

		req := httptest.NewRequest(http.MethodPost, "/v1/objects/upload", strings.NewReader(`{"bucket_name":"bucket-a","object_key":"uploads/a.jpg","content_type":"image/jpeg","content_b64":"%%%"}`))
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("returns not found when bucket connection is missing", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: true}}
		h := UploadObjectHandler(authz, &stubObjectUploadService{err: service.ErrBucketConnectionNotFound})

		req := httptest.NewRequest(http.MethodPost, "/v1/objects/upload", strings.NewReader(`{"bucket_name":"bucket-a","object_key":"uploads/a.jpg","content_type":"image/jpeg","content_b64":"cGF5bG9hZA=="}`))
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
	})

	t.Run("returns upstream failure for unexpected upload errors", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: true}}
		h := UploadObjectHandler(authz, &stubObjectUploadService{err: errors.New("s3 timeout")})

		req := httptest.NewRequest(http.MethodPost, "/v1/objects/upload", strings.NewReader(`{"bucket_name":"bucket-a","object_key":"uploads/a.jpg","content_type":"image/jpeg","content_b64":"cGF5bG9hZA=="}`))
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected status 502, got %d", rec.Code)
		}
	})
}

func TestPresignDownloadObjectHandler_UsesReadAction(t *testing.T) {
	authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: false, Reason: auth.DecisionReasonActionScope}}
	h := PresignDownloadObjectHandler(authz)

	claims := auth.Claims{Subject: "svc-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleProjectClient, PrincipalType: auth.PrincipalTypeService}
	req := httptest.NewRequest(http.MethodPost, "/v1/objects/presign-download", strings.NewReader(`{"bucket_name":"bucket-a","object_key":"images/a.jpg"}`))
	req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
	if authz.request.Action != auth.ActionRead {
		t.Fatalf("expected read action, got %q", authz.request.Action)
	}
}

func TestDeleteObjectHandler(t *testing.T) {
	claims := auth.Claims{Subject: "user-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleProjectClient, PrincipalType: auth.PrincipalTypeUser}

	t.Run("returns forbidden when authorization denies", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: false, Reason: auth.DecisionReasonPrefixScope}}
		h := DeleteObjectHandler(authz, nil)

		req := httptest.NewRequest(http.MethodDelete, "/v1/objects", strings.NewReader(`{"bucket_name":"bucket-a","object_key":"uploads/a.jpg"}`))
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", rec.Code)
		}
	})

	t.Run("returns not implemented when delete service is not configured", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: true}}
		h := DeleteObjectHandler(authz, nil)

		req := httptest.NewRequest(http.MethodDelete, "/v1/objects", strings.NewReader(`{"bucket_name":"bucket-a","object_key":"uploads/a.jpg"}`))
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("expected status 501, got %d", rec.Code)
		}
	})

	t.Run("returns ok when delete succeeds", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: true}}
		deleteSvc := &stubObjectDeleteService{result: service.ObjectDeleteResult{Deleted: true}}
		h := DeleteObjectHandler(authz, deleteSvc)

		req := httptest.NewRequest(http.MethodDelete, "/v1/objects", strings.NewReader(`{"bucket_name":"bucket-a","object_key":"uploads/a.jpg"}`))
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if deleteSvc.input.ProjectID != "project-1" || deleteSvc.input.AppID != "app-1" {
			t.Fatalf("unexpected delete scope: project=%s app=%s", deleteSvc.input.ProjectID, deleteSvc.input.AppID)
		}
	})

	t.Run("returns not found when bucket connection is missing", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: true}}
		h := DeleteObjectHandler(authz, &stubObjectDeleteService{err: service.ErrBucketConnectionNotFound})

		req := httptest.NewRequest(http.MethodDelete, "/v1/objects", strings.NewReader(`{"bucket_name":"bucket-a","object_key":"uploads/a.jpg"}`))
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
	})
}

func TestObjectHandlers_ReturnNotImplementedWhenAuthorized(t *testing.T) {
	claims := auth.Claims{Subject: "user-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleProjectClient, PrincipalType: auth.PrincipalTypeUser}

	tests := []struct {
		name       string
		method     string
		path       string
		handlerFn  func(AuthorizationService) http.HandlerFunc
		wantAction auth.Action
	}{
		{name: "presign upload", method: http.MethodPost, path: "/v1/objects/presign-upload", handlerFn: PresignUploadObjectHandler, wantAction: auth.ActionWrite},
		{name: "presign download", method: http.MethodPost, path: "/v1/objects/presign-download", handlerFn: PresignDownloadObjectHandler, wantAction: auth.ActionRead},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: true}}
			h := tc.handlerFn(authz)

			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"bucket_name":"bucket-a","object_key":"uploads/a.jpg"}`))
			req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("expected status 501, got %d", rec.Code)
			}

			var got apiErrorEnvelope
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if got.Error == nil || got.Error.Code != "not_implemented" {
				t.Fatalf("expected not_implemented error, got %+v", got.Error)
			}
			if authz.request.Action != tc.wantAction {
				t.Fatalf("expected action %q, got %q", tc.wantAction, authz.request.Action)
			}
		})
	}
}
