# S3 Storage Service — Build Plan

Standalone service used by Locket (and future projects) to securely access multiple S3 buckets.

---

## Progress overview

| Epic      | Title                                  | Tasks | Points |
| --------- | -------------------------------------- | ----- | ------ |
| 1         | Service foundation                     | 8     | 13     |
| 2         | Auth and access control                | 8     | 18     |
| 3         | Multi-bucket S3 orchestration          | 8     | 16     |
| 4         | Storage API surface                    | 10    | 15     |
| 5         | Daily cost notifier (Discord)          | 5     | 10     |
| 6         | Hardening, observability, and deploy   | 9     | 14     |
| **Total** |                                        | **48**| **86** |

---

## Epic 1 — Service foundation

> Repo scaffold · runtime config · persistence

- [x] **1.1** New Go service scaffold with router, structured logging, health endpoint `Go` · 2 pts
- [x] **1.2** Config loader for env + secrets manager references `Go` `Infra` · 2 pts
- [x] **1.3** Postgres schema: bucket_connections, access_policies, audit_events `DB` · 2 pts
- [x] **1.4** Migration runner + boot-time schema check `Go` `DB` · 2 pts
- [x] **1.5** Request ID middleware and response error envelope standard `Go` · 1 pt
- [x] **1.6** Local docker-compose for service + postgres + optional localstack `Infra` · 1 pt
- [ ] **1.7** Baseline CI workflow (lint, test, build) `Infra` · 1 pt

---

## Epic 2 — Auth and access control

> JWT validation · client auth · per-bucket authorization

- [ ] **2.1** JWT middleware: validate issuer, audience, expiry, signature `Go` `Auth` · 3 pts
- [ ] **2.2** Role model: admin, project-client, read-only-client `Go` `Auth` `DB` · 2 pts
- [ ] **2.3** Bucket connection ownership model by project/client ID and app principal (`app_id`) `Go` `DB` · 2 pts
- [ ] **2.4** Prefix-level authorization (allow listed prefixes only) with per-app action scope (read/write/delete/list) `Go` `Auth` · 3 pts
- [ ] **2.5** Service-to-service auth option (OAuth2 client credentials) for backend callers `Go` `Auth` · 2 pts
- [ ] **2.6** Security audit log for every upload/delete/presign call `Go` `DB` · 2 pts
- [ ] **2.7** Rate limit by caller identity + IP for abuse protection `Go` `Infra` · 2 pts
- [ ] **2.8** Auth integration tests for invalid token, wrong audience, forbidden bucket/prefix, and app scope denial `Go` `Auth` · 2 pts

---

## Epic 3 — Multi-bucket S3 orchestration

> AssumeRole flow · temporary creds · private buckets only

- [ ] **3.1** Store bucket connection metadata: bucket name, region, role ARN, external ID, allowed prefixes `Go` `DB` · 2 pts
- [ ] **3.2** STS AssumeRole client with cached short-lived sessions `Go` `S3` · 3 pts
- [ ] **3.3** Enforce private bucket baseline and object ownership settings checks `Go` `S3` · 2 pts
- [ ] **3.4** Upload helper with content type validation, max size checks, metadata support `Go` `S3` · 2 pts
- [ ] **3.5** Delete helper with prefix guardrails and soft-fail idempotency `Go` `S3` · 2 pts
- [ ] **3.6** Presign helper for GET and PUT with short expirations `Go` `S3` · 2 pts
- [ ] **3.7** Retry strategy for throttling and transient AWS errors `Go` `S3` · 2 pts
- [ ] **3.8** Integration tests against real AWS test bucket path or localstack fallback `Go` `S3` · 1 pt

---

## Epic 4 — Storage API surface

> Stable contract for Locket and future clients

- [ ] **4.1** POST /v1/bucket-connections (create or register client bucket) `Go` `API` · 2 pts
- [ ] **4.2** GET /v1/bucket-connections (list by authenticated project/client) `Go` `API` · 1 pt
- [ ] **4.3** POST /v1/objects/upload (server-side upload) `Go` `API` · 2 pts
- [ ] **4.4** POST /v1/objects/presign-upload (browser direct upload) `Go` `API` · 2 pts
- [ ] **4.5** GET /v1/images/:id (authenticated image stream via backend, enforce `app_id` + ownership policy) `Go` `API` · 2 pts
- [ ] **4.6** GET /v1/images (authenticated image list with metadata + backend URL) `Go` `API` · 1 pt
- [ ] **4.7** DELETE /v1/images/:id (safe delete with ownership check and app delete scope) `Go` `API` · 1 pt
- [ ] **4.8** POST /v1/objects/presign-download (optional/admin-only short-lived read links) `Go` `API` · 1 pt
- [ ] **4.9** Unified error codes (auth_failed, forbidden, not_found, throttle, upstream_failure) `Go` `API` · 1 pt
- [ ] **4.10** API docs (OpenAPI + examples for Locket integration) `Infra` `Docs` · 2 pts

---

## Epic 5 — Daily cost notifier (Discord)

> Monthly budget already configured · daily scheduler · Discord daily digest

- [x] **5.1** AWS Budget monthly threshold alerts configured (notify only baseline) `FinOps` · 1 pt
- [ ] **5.2** Notification channel integration: Discord webhook for daily cost digest `Infra` `FinOps` · 2 pts
- [ ] **5.3** Droplet scheduled billing checker as a separate command/job (cron or systemd timer) running every day, querying Cost Explorer APIs `Go` `FinOps` · 3 pts
- [ ] **5.4** Daily digest payload and delivery: yesterday, month-to-date, forecast, top services, highest-growth service (send to Discord daily) `Go` `FinOps` · 2 pts
- [ ] **5.5** Notifier reliability: job lock, retries, and dedupe for daily Discord sends `Go` `FinOps` · 2 pts

---

## Epic 6 — Hardening, observability, and deploy

> Security defaults · monitoring · resilient ops

- [ ] **6.1** Deploy baseline on droplet or container host with TLS and reverse proxy `Infra` · 2 pts
- [ ] **6.2** Secrets isolation for JWT keys, OAuth client secret, and AWS bootstrap creds `Infra` `Auth` · 2 pts
- [ ] **6.9** Startup contract: run migrations during deploy/start (`migrate` before `serve`) and fail fast on migration errors `Go` `Infra` · 1 pt
- [ ] **6.3** Structured metrics: request latency, auth failures, upstream S3 errors, presign counts `Infra` · 2 pts
- [ ] **6.4** Alerting on error rate, auth failure spike, and STS assume-role failure `Infra` · 2 pts
- [ ] **6.5** Notifier reliability: job lock, retries, and failed-email queue handling on droplet `Infra` · 1 pt
- [ ] **6.6** Disaster recovery checklist for DB backups and config restore `Infra` `DB` · 1 pt
- [ ] **6.7** Load test for upload and presign endpoints with rate-limit verification `Infra` · 2 pts
- [ ] **6.8** Release checklist and rollback plan `Infra` `Docs` · 1 pt

---

## Build order

```
Epic 1 (Service foundation)
  └─▶ Epic 2 (Auth and access control)
        └─▶ Epic 3 (Multi-bucket S3 orchestration)
              └─▶ Epic 4 (Storage API surface)
                    └─▶ Epic 5 (Daily cost notifier (Discord))
                          └─▶ Epic 6 (Hardening, observability, and deploy)
```

## Watch out for

- **2.1** JWT audience and issuer mismatch is the most common source of false 401 responses in service-to-service auth.
- **3.2** STS role trust policy and external ID drift can silently block bucket access in production.
- **5.3** Cost Explorer data freshness can lag; daily Discord summaries are near-daily visibility, not real-time accounting.
- **5.5** Cron overlap can duplicate daily Discord sends; enforce single-instance lock and dedupe key.
- **6.5** Cron overlap can send duplicate notices; add lock files or systemd single-instance protections.

---

## Secure baseline profile (recommended for your two-user app)

Use private S3 buckets with backend-authorized access only. This keeps the model simple, secure, and easy to extend later.

### Required security controls

- Bucket controls:
      - Block all public access at bucket and account level.
      - Do not create bucket policies that allow anonymous `s3:GetObject`.
      - Enforce server-side encryption (SSE-S3 at minimum, SSE-KMS if needed later).
- Backend controls:
      - Require authenticated sessions/tokens for every image route.
      - Resolve caller app identity (`app_id`) from token and enforce app policy before S3 calls.
      - Authorize object access by owner/project fields before reading from S3.
      - Use IAM role or scoped access keys only on the backend (never in frontend).
- Database controls:
      - Store object keys and metadata only; never store AWS credentials.
      - Include ownership fields (`owner_user_id`, `project_id`, `visibility`) for future multi-user growth.
- Observability:
      - Enable CloudWatch request/error metrics for S3.
      - Add low-cost AWS budget alarms to catch accidental usage spikes.

### Endpoint changes for Option 1 (backend streams images)

For your use case, prefer backend streaming and avoid exposing direct S3 download links.

- Keep / add:
      - `POST /v1/objects/upload` (authenticated upload via backend)
      - `GET /v1/images/:id` (authenticated image stream from backend)
      - `GET /v1/images` (authenticated list with metadata)
      - `DELETE /v1/images/:id` (authenticated delete with ownership check)
- Change:
      - Make `POST /v1/objects/presign-download` optional or admin-only. Default should be disabled.
      - If download links are needed for sharing later, issue very short-lived links and log all issuance.
- Do not add:
      - Public image endpoints (for example: `GET /public/images/:id`).

### API response shape for image URLs

To support frontend `<img>` tags while staying private, return your backend URL (not a raw S3 URL).

- Example `GET /v1/images` item:

```json
{
      "id": "img_123",
      "owner_user_id": "user_1",
      "mime_type": "image/jpeg",
      "size_bytes": 183442,
      "created_at": "2026-03-22T19:20:00Z",
      "url": "/v1/images/img_123"
}
```

This gives the frontend a stable URL while the backend still performs auth + S3 retrieval.

### Mapping to current tasks

- Add a task in Epic 4 for `GET /v1/images/:id` streaming endpoint.
- Re-scope **4.5** (`presign-download`) to optional/non-default mode.
- Add ownership checks to read/delete handlers so auth rules are enforced consistently.

---

_Recommended for shared internal platform use, with Locket as first client._
