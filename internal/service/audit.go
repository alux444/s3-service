package service

import (
	"context"

	"s3-service/internal/database"
)

type AuditEventRepository interface {
	RecordEvent(ctx context.Context, event database.AuditEventWrite) error
}

type AuditService struct {
	repo AuditEventRepository
}

func NewAuditService(repo AuditEventRepository) *AuditService {
	return &AuditService{repo: repo}
}

func (s *AuditService) RecordEvent(ctx context.Context, event database.AuditEventWrite) error {
	return s.repo.RecordEvent(ctx, event)
}
