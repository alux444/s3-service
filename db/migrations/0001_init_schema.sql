-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE bucket_connections (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id TEXT NOT NULL,
  app_id TEXT NOT NULL,
  bucket_name TEXT NOT NULL,
  region TEXT NOT NULL,
  role_arn TEXT NOT NULL,
  external_id TEXT,
  allowed_prefixes TEXT[] NOT NULL DEFAULT '{}',
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_bucket_connections_project_app_bucket
    UNIQUE (project_id, app_id, bucket_name)
);

COMMENT ON TABLE bucket_connections IS
  'Registered S3 bucket integrations owned by a project/app principal.';
COMMENT ON COLUMN bucket_connections.id IS
  'Primary key UUID for the bucket connection record.';
COMMENT ON COLUMN bucket_connections.project_id IS
  'Tenant/project identifier owning this bucket connection.';
COMMENT ON COLUMN bucket_connections.app_id IS
  'Application principal identifier used for authz and ownership checks.';
COMMENT ON COLUMN bucket_connections.bucket_name IS
  'Target S3 bucket name.';
COMMENT ON COLUMN bucket_connections.region IS
  'AWS region for the target bucket.';
COMMENT ON COLUMN bucket_connections.role_arn IS
  'IAM role ARN assumed via STS to access this bucket.';
COMMENT ON COLUMN bucket_connections.external_id IS
  'Optional STS ExternalId required by role trust policy.';
COMMENT ON COLUMN bucket_connections.allowed_prefixes IS
  'Global allowed S3 key prefixes for this connection; empty means no prefix access.';
COMMENT ON COLUMN bucket_connections.is_active IS
  'Soft-toggle to disable a connection without deleting history.';
COMMENT ON COLUMN bucket_connections.created_at IS
  'Creation timestamp in UTC.';
COMMENT ON COLUMN bucket_connections.updated_at IS
  'Last update timestamp in UTC.';
COMMENT ON CONSTRAINT uq_bucket_connections_project_app_bucket ON bucket_connections IS
  'Prevents duplicate bucket registrations per project/app.';
  
CREATE INDEX idx_bucket_connections_project_app
  ON bucket_connections (project_id, app_id);

COMMENT ON INDEX idx_bucket_connections_project_app IS
  'Speeds lookups of bucket connections by project and app principal.';

CREATE TABLE access_policies (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  bucket_connection_id UUID NOT NULL REFERENCES bucket_connections(id) ON DELETE CASCADE,
  principal_type TEXT NOT NULL CHECK (principal_type IN ('user', 'service')),
  principal_id TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('admin', 'project-client', 'read-only-client')),
  can_read BOOLEAN NOT NULL DEFAULT TRUE,
  can_write BOOLEAN NOT NULL DEFAULT FALSE,
  can_delete BOOLEAN NOT NULL DEFAULT FALSE,
  can_list BOOLEAN NOT NULL DEFAULT TRUE,
  prefix_allowlist TEXT[] NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_access_policies_connection_principal
    UNIQUE (bucket_connection_id, principal_type, principal_id)
);

COMMENT ON TABLE access_policies IS
  'Per-principal authorization rules for each bucket connection.';
COMMENT ON COLUMN access_policies.id IS
  'Primary key UUID for the policy record.';
COMMENT ON COLUMN access_policies.bucket_connection_id IS
  'Foreign key to bucket_connections; cascades on connection delete.';
COMMENT ON COLUMN access_policies.principal_type IS
  'Type of principal receiving permissions: user or service.';
COMMENT ON COLUMN access_policies.principal_id IS
  'Unique subject identifier from identity provider token/claims.';
COMMENT ON COLUMN access_policies.role IS
  'High-level role used for business authorization decisions.';
COMMENT ON COLUMN access_policies.can_read IS
  'Permission to read/download object data.';
COMMENT ON COLUMN access_policies.can_write IS
  'Permission to upload/write object data.';
COMMENT ON COLUMN access_policies.can_delete IS
  'Permission to delete objects.';
COMMENT ON COLUMN access_policies.can_list IS
  'Permission to list object keys/metadata.';
COMMENT ON COLUMN access_policies.prefix_allowlist IS
  'Per-principal allowed S3 key prefixes; intersects with connection prefixes.';
COMMENT ON COLUMN access_policies.created_at IS
  'Creation timestamp in UTC.';
COMMENT ON COLUMN access_policies.updated_at IS
  'Last update timestamp in UTC.';
COMMENT ON CONSTRAINT uq_access_policies_connection_principal ON access_policies IS
  'Prevents duplicate policy entries for same principal on same connection.';

CREATE INDEX idx_access_policies_principal
  ON access_policies (principal_type, principal_id);

COMMENT ON INDEX idx_access_policies_principal IS
  'Speeds policy lookups by authenticated principal identity.';

CREATE TABLE audit_events (
  id BIGSERIAL PRIMARY KEY,
  request_id TEXT,
  event_type TEXT NOT NULL,
  actor_type TEXT NOT NULL CHECK (actor_type IN ('user', 'service', 'system')),
  actor_id TEXT,
  project_id TEXT,
  app_id TEXT,
  bucket_connection_id UUID REFERENCES bucket_connections(id) ON DELETE SET NULL,
  bucket_name TEXT,
  object_key TEXT,
  action TEXT NOT NULL,
  outcome TEXT NOT NULL CHECK (outcome IN ('allowed', 'denied', 'error')),
  http_status INTEGER,
  error_code TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE audit_events IS
  'Immutable audit trail for authz decisions and storage operations.';
COMMENT ON COLUMN audit_events.id IS
  'Monotonic event identifier for ordering and pagination.';
COMMENT ON COLUMN audit_events.request_id IS
  'Request correlation ID from middleware for traceability.';
COMMENT ON COLUMN audit_events.event_type IS
  'Domain-specific event category (e.g., upload_requested, delete_denied).';
COMMENT ON COLUMN audit_events.actor_type IS
  'Originator type: user, service, or system.';
COMMENT ON COLUMN audit_events.actor_id IS
  'Identity of the originating principal when available.';
COMMENT ON COLUMN audit_events.project_id IS
  'Project/tenant context at event time.';
COMMENT ON COLUMN audit_events.app_id IS
  'Application principal context at event time.';
COMMENT ON COLUMN audit_events.bucket_connection_id IS
  'Optional FK to bucket connection; nulled if connection is deleted.';
COMMENT ON COLUMN audit_events.bucket_name IS
  'Resolved bucket name used in the operation.';
COMMENT ON COLUMN audit_events.object_key IS
  'Target S3 object key/path involved in the event.';
COMMENT ON COLUMN audit_events.action IS
  'Operation attempted (upload, delete, presign, list, etc.).';
COMMENT ON COLUMN audit_events.outcome IS
  'Final result: allowed, denied, or error.';
COMMENT ON COLUMN audit_events.http_status IS
  'HTTP status returned by this service for the operation.';
COMMENT ON COLUMN audit_events.error_code IS
  'Stable internal error code for failure/denial classification.';
COMMENT ON COLUMN audit_events.metadata IS
  'Flexible JSON diagnostics (AWS error, latency, user-agent, extra context).';
COMMENT ON COLUMN audit_events.occurred_at IS
  'Event timestamp in UTC.';
COMMENT ON CONSTRAINT audit_events_actor_type_check ON audit_events IS
  'Restricts actor_type to supported values.';
COMMENT ON CONSTRAINT audit_events_outcome_check ON audit_events IS
  'Restricts outcome to supported values.';

CREATE INDEX idx_audit_events_occurred_at
  ON audit_events (occurred_at DESC);

COMMENT ON INDEX idx_audit_events_occurred_at IS
  'Speeds reverse-chronological audit queries.';

CREATE INDEX idx_audit_events_project_app_time
  ON audit_events (project_id, app_id, occurred_at DESC);

COMMENT ON INDEX idx_audit_events_project_app_time IS
  'Speeds scoped audit queries by project/app with recent-first ordering.';

-- +goose Down
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS access_policies;
DROP TABLE IF EXISTS bucket_connections;