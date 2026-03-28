package handlers

import (
	"context"

	"s3-service/internal/auth"
)

type AuthorizationService interface {
	Authorize(ctx context.Context, claims auth.Claims, bucketName string, action auth.Action, objectKey string) auth.Decision
}
