package service

import (
	"context"
	"errors"
	"testing"

	"s3-service/internal/database"
)

type stubAuditRepo struct {
	event database.AuditEventWrite
	err   error
}

func (s *stubAuditRepo) RecordEvent(_ context.Context, event database.AuditEventWrite) error {
	s.event = event
	return s.err
}

func TestAuditService_RecordEvent(t *testing.T) {
	t.Run("forwards audit event to repository", func(t *testing.T) {
		repo := &stubAuditRepo{}
		svc := NewAuditService(repo)

		event := database.AuditEventWrite{
			RequestID: "req-1",
			EventType: "upload_requested",
			ActorType: "service",
			ActorID:   "svc-images",
			ProjectID: "project-1",
			AppID:     "app-1",
			Action:    "upload",
			Outcome:   "allowed",
		}

		if err := svc.RecordEvent(context.Background(), event); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if repo.event.RequestID != "req-1" || repo.event.EventType != "upload_requested" {
			t.Fatalf("unexpected event forwarded: %+v", repo.event)
		}
	})

	t.Run("returns repository error", func(t *testing.T) {
		repo := &stubAuditRepo{err: errors.New("insert failed")}
		svc := NewAuditService(repo)

		err := svc.RecordEvent(context.Background(), database.AuditEventWrite{
			EventType: "delete_requested",
			ActorType: "user",
			Action:    "delete",
			Outcome:   "error",
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
