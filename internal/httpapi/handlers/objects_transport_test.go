package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"s3-service/internal/auth"
	httpmiddleware "s3-service/internal/httpapi/middleware"
)

type stubObjectAuthorizationService struct {
	decision auth.Decision
	request  auth.AuthorizationRequest
}

func (s *stubObjectAuthorizationService) Authorize(_ context.Context, request auth.AuthorizationRequest) auth.Decision {
	s.request = request
	return s.decision
}

type objectHandlerResponse struct {
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details *struct {
			Reason string `json:"reason"`
		} `json:"details"`
	} `json:"error"`
}

func TestUploadObjectHandler(t *testing.T) {
	claims := auth.Claims{Subject: "user-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleProjectClient, PrincipalType: auth.PrincipalTypeUser}

	t.Run("returns forbidden when authorization denies", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: false, Reason: auth.DecisionReasonPrefixScope}}
		h := UploadObjectHandler(authz)

		req := httptest.NewRequest(http.MethodPost, "/v1/objects/upload", strings.NewReader(`{"bucket_name":"bucket-a","object_key":"uploads/a.jpg"}`))
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

	t.Run("returns not implemented when authorization allows", func(t *testing.T) {
		authz := &stubObjectAuthorizationService{decision: auth.Decision{Allowed: true}}
		h := UploadObjectHandler(authz)

		req := httptest.NewRequest(http.MethodPost, "/v1/objects/upload", strings.NewReader(`{"bucket_name":"bucket-a","object_key":"uploads/a.jpg"}`))
		req = req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("expected status 501, got %d", rec.Code)
		}
		var got objectHandlerResponse
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.Error == nil || got.Error.Code != "not_implemented" {
			t.Fatalf("expected not_implemented error, got %+v", got.Error)
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

func TestObjectHandlers_ReturnNotImplementedWhenAuthorized(t *testing.T) {
	claims := auth.Claims{Subject: "user-1", AppID: "app-1", ProjectID: "project-1", Role: auth.RoleProjectClient, PrincipalType: auth.PrincipalTypeUser}

	tests := []struct {
		name       string
		method     string
		path       string
		handlerFn  func(AuthorizationService) http.HandlerFunc
		wantAction auth.Action
	}{
		{name: "delete", method: http.MethodDelete, path: "/v1/objects", handlerFn: DeleteObjectHandler, wantAction: auth.ActionDelete},
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

			var got objectHandlerResponse
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
