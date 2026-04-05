# Module Functionality Playbook

This guide explains how to add new functionality in this codebase using the current architecture:

- HTTP transport in `internal/httpapi/handlers` and `internal/httpapi/router`
- Business orchestration in `internal/service`
- Integrations/adapters in `internal/database` and `internal/s3`
- Wiring/composition in `cmd/s3-service/main.go`

Use this as a repeatable checklist.

## 1) Define the behavior first

Before coding, write down:

- What endpoint or trigger starts the flow
- Required inputs and validation rules
- Business rules and failure cases
- Which external systems are involved (DB, AWS)
- Expected output and HTTP status/error mapping

Example statement:

"Add an endpoint that validates a bucket and saves metadata only if it passes security baseline checks."

## 2) Choose the correct layer for each responsibility

Split by responsibility, not by feature name.

- Handler: parse request, auth context, call service, map errors to API response
- Service: business rules, orchestration, call dependencies
- Adapter/module: concrete DB/AWS calls
- Router: register endpoint/middleware
- Main: build concrete objects and inject dependencies

Rule of thumb:

- If it mentions JSON, status code, headers -> handler
- If it mentions policy/rules/decision -> service
- If it mentions SQL/AWS SDK -> adapter module

## 3) Add or update domain interfaces in service

Start from `internal/service` and define what the service needs, not how it is implemented.

- Add dependency interfaces (validator/repository/client)
- Add service options for optional behavior
- Keep service constructor injectable and testable

Why:

- You can mock dependencies in tests
- Main can compose different implementations without changing service logic

## 4) Implement integration logic in adapter modules

Put concrete integrations behind focused modules:

- DB code in `internal/database`
- AWS code in `internal/s3`

Keep adapter functions narrow:

- Input in domain terms
- Return typed errors for decision-making upstream
- Avoid HTTP concerns in adapters

## 5) Wire transport (handler + router)

In handler:

- Decode request
- Validate required fields
- Read claims/context
- Call service
- Convert domain errors to API error codes/messages

In router:

- Register route under the right version group
- Attach auth/rate-limit/audit middleware as needed

## 6) Wire dependencies in main (composition root)

In `cmd/s3-service/main.go`:

- Create concrete adapters/modules
- Create services with injected dependencies/options
- Pass services into router

All concrete construction should happen here so business code stays clean.

## 7) Add tests at each layer

Minimum set:

- Adapter tests: integration behavior and error classification
- Service tests: rule enforcement + dependency interactions
- Handler transport tests: request/response status and error envelope mapping

Test pyramid for new functionality:

1. Service unit tests (fast, most logic)
2. Handler transport tests (contract behavior)
3. Adapter tests (SDK/SQL edge cases)

## 8) Add/refresh docs and task tracking

- Update `s3-service.md` task status
- Document any new API code paths or constraints
- If needed, add an operational note (env vars, IAM requirements)

## 9) Validate with focused commands

Typical validation loop:

1. `gofmt -w <changed files>`
2. `go test ./internal/<affected package> ...`
3. `go test ./...` (optional before merge)

## End-to-end example: Add bucket security baseline enforcement during bucket connection create

This is the exact pattern used for task 3.3.

### Step A: Add an adapter module for S3 checks

Create `internal/s3/bucket_baseline.go` that:

- Uses an assumed-role config provider
- Calls S3 APIs:
  - `GetPublicAccessBlock`
  - `GetBucketPolicyStatus`
  - `GetBucketOwnershipControls`
- Returns a typed violation error if baseline fails

### Step B: Add service dependency hook

Update `internal/service/bucket_connections.go`:

- Add `BucketConnectionSecurityValidator` interface
- Add `WithBucketConnectionSecurityValidator(...)` service option
- Call validator in `CreateForScope(...)` before repository save

### Step C: Map domain error to HTTP error

Update `internal/httpapi/handlers/bucket_connections.go`:

- Detect baseline violation errors
- Return `400` with a stable API code (for example, `bucket_security_baseline_failed`)

### Step D: Wire implementation in main

Update `cmd/s3-service/main.go`:

- Build `AssumeRoleSessionCache`
- Build `BucketSecurityBaselineChecker`
- Inject checker into `NewBucketConnectionsService(...)`

### Step E: Test all layers

- `internal/s3/bucket_baseline_test.go`: secure vs insecure bucket cases
- `internal/service/bucket_connections_test.go`: validator called, save skipped on failure
- `internal/httpapi/handlers/bucket_connections_transport_test.go`: HTTP 400 mapping

### Step F: Mark task and verify

- Mark task complete in `s3-service.md`
- Run focused tests and ensure green

## Quick checklist (copy/paste)

- [ ] Behavior and error cases defined
- [ ] Service interfaces/options updated
- [ ] Adapter module implemented (DB/AWS)
- [ ] Handler parses/maps errors correctly
- [ ] Router route/middleware registration done
- [ ] Main wiring/injection updated
- [ ] Unit + transport tests added
- [ ] Task/docs updated
- [ ] gofmt and go test pass
