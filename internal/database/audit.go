package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type AuditEventWrite struct {
	RequestID          string
	EventType          string
	ActorType          string
	ActorID            string
	ProjectID          string
	AppID              string
	BucketConnectionID *string
	BucketName         string
	ObjectKey          string
	Action             string
	Outcome            string
	HTTPStatus         int
	ErrorCode          string
	Metadata           map[string]any
}

type AuditRepository struct {
	db *sql.DB
}

func NewAuditRepository(db *sql.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) RecordEvent(ctx context.Context, event AuditEventWrite) error {
	return RecordAuditEvent(ctx, r.db, event)
}

func RecordAuditEvent(ctx context.Context, db *sql.DB, event AuditEventWrite) error {
	if event.EventType == "" || event.ActorType == "" || event.Action == "" || event.Outcome == "" {
		return fmt.Errorf("eventType, actorType, action, and outcome must be provided")
	}

	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal audit metadata: %w", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO audit_events (
			request_id,
			event_type,
			actor_type,
			actor_id,
			project_id,
			app_id,
			bucket_connection_id,
			bucket_name,
			object_key,
			action,
			outcome,
			http_status,
			error_code,
			metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
	`,
		event.RequestID,
		event.EventType,
		event.ActorType,
		event.ActorID,
		event.ProjectID,
		event.AppID,
		event.BucketConnectionID,
		event.BucketName,
		event.ObjectKey,
		event.Action,
		event.Outcome,
		event.HTTPStatus,
		event.ErrorCode,
		metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to insert audit event: %w", err)
	}

	return nil
}
