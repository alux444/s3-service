package handlers

import (
	"context"

	"s3-service/internal/auth"
)

type AuthorizationService interface {
	Authorize(ctx context.Context, request auth.AuthorizationRequest) auth.Decision
}
