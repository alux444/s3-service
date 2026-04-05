package handlers

import (
	"context"

	"s3-service/internal/service"
)

// objectPresignRequest is a staging contract for task 3.6 presign implementation.
//
// WISHFUL THINKING (3.6):
// - Decide whether to keep this unified for GET/PUT or split request shapes per endpoint.
// - Validate method-specific fields (content_type relevant for PUT only).
// - Add explicit expires_in_seconds input with bounded limits.
type objectPresignRequest struct {
	BucketName       string `json:"bucket_name"`
	ObjectKey        string `json:"object_key"`
	Method           string `json:"method"` // GET or PUT
	ExpiresInSeconds int64  `json:"expires_in_seconds,omitempty"`
	ContentType      string `json:"content_type,omitempty"`
}

type objectPresignResponse struct {
	Method    string `json:"method"`
	URL       string `json:"url"`
	ExpiresIn int64  `json:"expires_in_seconds"`
}

type ObjectPresignService interface {
	PresignObject(ctx context.Context, input service.ObjectPresignInput) (service.ObjectPresignResult, error)
}
