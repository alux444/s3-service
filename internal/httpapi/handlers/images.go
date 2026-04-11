package handlers

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"s3-service/internal/auth"
	"s3-service/internal/httpapi"
	"s3-service/internal/s3"
	"s3-service/internal/service"
)

type ObjectReadService interface {
	ReadObject(ctx context.Context, input service.ObjectReadInput) (service.ObjectReadResult, error)
}

func GetImageHandler(authorizationService AuthorizationService, readService ObjectReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsOrUnauthorized(w, r)
		if !ok {
			return
		}

		bucketName, objectKey, err := decodeImageID(chi.URLParam(r, "id"))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid image id", httpapi.ValidationDetails{Field: "id", Reason: "invalid_format"})
			return
		}

		decision := authorizationService.Authorize(r.Context(), auth.AuthorizationRequest{
			Claims:     claims,
			BucketName: bucketName,
			Action:     auth.ActionRead,
			ObjectKey:  objectKey,
		})
		if !decision.Allowed {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "operation not permitted for this scope", httpapi.AuthDetails{Reason: decision.Reason})
			return
		}

		if readService == nil {
			httpapi.WriteError(w, r, http.StatusNotImplemented, "not_implemented", "image read is not implemented yet", nil)
			return
		}

		result, err := readService.ReadObject(r.Context(), service.ObjectReadInput{
			ProjectID:  claims.ProjectID,
			AppID:      claims.AppID,
			BucketName: bucketName,
			ObjectKey:  objectKey,
		})
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidObjectReadInput), errors.Is(err, s3.ErrReadPrefixGuardrailViolation), errors.Is(err, s3.ErrInvalidAssumeRoleInput):
				httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_request", err.Error(), httpapi.ValidationDetails{Field: "image", Reason: "invalid_input"})
				return
			case errors.Is(err, service.ErrBucketConnectionNotFound):
				httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "bucket connection not found for scope", httpapi.NotFoundDetails{Resource: "bucket_connection", ID: bucketName})
				return
			case errors.Is(err, s3.ErrObjectNotFound):
				httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "image not found", httpapi.NotFoundDetails{Resource: "object", ID: objectKey})
				return
			default:
				httpapi.WriteError(w, r, http.StatusBadGateway, "upstream_failure", "failed to read image from storage provider", nil)
				return
			}
		}
		defer result.Body.Close()

		if result.ContentType != "" {
			w.Header().Set("Content-Type", result.ContentType)
		} else {
			w.Header().Set("Content-Type", "application/octet-stream")
		}
		if result.ContentLength >= 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(result.ContentLength, 10))
		}
		if result.ETag != "" {
			w.Header().Set("ETag", result.ETag)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, result.Body)
	}
}

func decodeImageID(id string) (bucketName string, objectKey string, err error) {
	if strings.TrimSpace(id) == "" {
		return "", "", errors.New("image id is required")
	}

	raw, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return "", "", err
	}

	parts := strings.SplitN(string(raw), ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", errors.New("invalid image id format")
	}

	return parts[0], parts[1], nil
}
