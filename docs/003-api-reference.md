# S3 Service API Reference (Minimal and Complete)

This document is intended to be enough to use the API without reading source code.

## Base URL

- Local default: `http://localhost:8080`

Set once in shell:

```bash
export BASE_URL="http://localhost:8080"
export TOKEN="<jwt>"
```

## Authentication

All `/v1/*` endpoints require:

- Header: `Authorization: Bearer <JWT>`

Required claims used by the service:

- `sub`
- `app_id`
- `project_id`
- `role`
- `principal_type`

If auth fails, response code is `401` with error code `auth_failed`.

## Rate Limiting

`/v1/*` endpoints are rate limited by identity + client IP.

- HTTP status: `429`
- Error code: `throttle`
- Headers:
  - `X-RateLimit-Limit`
  - `X-RateLimit-Remaining`
  - `Retry-After`

## Response Format

Most JSON success responses:

```json
{
  "data": { }
}
```

Error responses:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "...",
    "requestId": "...",
    "details": { }
  }
}
```

Image streaming endpoint (`GET /v1/images/{id}`) returns raw bytes, not a JSON envelope.

## Error Code Cheat Sheet

- `auth_failed`: missing/invalid/expired token
- `forbidden`: authenticated but not authorized for scope/action
- `not_found`: route or scoped resource missing
- `throttle`: rate limit exceeded
- `upstream_failure`: dependency/provider failure (for example AWS call failure)
- `invalid_request`: request body/fields/format invalid
- `not_implemented`: dependency not wired in runtime

## Image ID Format

`{id}` in `/v1/images/{id}` is Base64 URL-safe encoding of:

`<bucket_name>:<object_key>`

Example (portable via Python):

```bash
python3 - <<'PY'
import base64
raw = b"bucket-a:images/cat.jpg"
print(base64.urlsafe_b64encode(raw).decode().rstrip("="))
PY
```

## Endpoints

### 1) Health Check

`GET /health`

```bash
curl -s "$BASE_URL/health"
```

Example:

```json
{
  "data": {
    "status": "ok"
  }
}
```

### 2) Auth Check

`GET /v1/auth-check`

```bash
curl -s "$BASE_URL/v1/auth-check" \
  -H "Authorization: Bearer $TOKEN"
```

Example:

```json
{
  "data": {
    "sub": "user-1",
    "app_id": "app-1",
    "project_id": "project-1",
    "role": "project_client",
    "principal_type": "user"
  }
}
```

### 3) Create Bucket Connection

`POST /v1/bucket-connections`

```bash
curl -s -X POST "$BASE_URL/v1/bucket-connections" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_name": "bucket-a",
    "region": "us-east-1",
    "role_arn": "arn:aws:iam::123456789012:role/s3-runtime-role",
    "external_id": "runtime-external-id",
    "allowed_prefixes": ["images/", "uploads/private/"]
  }'
```

Success (`201`):

```json
{
  "data": {
    "created": true
  }
}
```

### 4) List Bucket Connections

`GET /v1/bucket-connections`

```bash
curl -s "$BASE_URL/v1/bucket-connections" \
  -H "Authorization: Bearer $TOKEN"
```

Success (`200`):

```json
{
  "data": {
    "buckets": [
      {
        "bucket_name": "bucket-a",
        "region": "us-east-1",
        "role_arn": "arn:aws:iam::123456789012:role/s3-runtime-role",
        "external_id": "runtime-external-id",
        "allowed_prefixes": ["images/", "uploads/private/"]
      }
    ]
  }
}
```

  ### 5) Upsert Access Policy (Admin Only)

  `POST /v1/access-policies`

  This endpoint creates or updates one access policy row for the current claim scope (`project_id` + `app_id`).

  Important:

  - Caller role must be `admin`.
  - `bucket_name` must already exist in this scope via `/v1/bucket-connections`.
  - If `can_*` fields are omitted, defaults are:
    - `can_read=true`
    - `can_write=false`
    - `can_delete=false`
    - `can_list=true`

  ```bash
  curl -s -X POST "$BASE_URL/v1/access-policies" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
      "bucket_name": "bucket-a",
      "principal_type": "service",
      "principal_id": "auth0|my-service-client-id",
      "role": "admin",
      "can_read": true,
      "can_write": true,
      "can_delete": true,
      "can_list": true,
      "prefix_allowlist": ["uploads/", "images/"]
    }'
  ```

  Success (`200`):

  ```json
  {
    "data": {
      "upserted": true
    }
  }
  ```

  ### 6) Upload Object (Server-Side)

`POST /v1/objects/upload`

```bash
curl -s -X POST "$BASE_URL/v1/objects/upload" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_name": "bucket-a",
    "object_key": "images/cat.jpg",
    "content_type": "image/jpeg",
    "content_b64": "aGVsbG8=",
    "metadata": {
      "source": "api-example"
    }
  }'
```

Success (`201`):

```json
{
  "data": {
    "uploaded": true,
    "bucket": "bucket-a",
    "object_key": "images/cat.jpg",
    "etag": "\"abc123\"",
    "size": 5
  }
}
```

### 7) Delete Object (Generic)

`DELETE /v1/objects`

```bash
curl -s -X DELETE "$BASE_URL/v1/objects" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_name": "bucket-a",
    "object_key": "images/cat.jpg"
  }'
```

Success (`200`):

```json
{
  "data": {
    "deleted": true,
    "bucket": "bucket-a",
    "object_key": "images/cat.jpg"
  }
}
```

### 8) Presign Upload URL

`POST /v1/objects/presign-upload`

`expires_in_seconds` is optional. Service enforces min/max bounds.

```bash
curl -s -X POST "$BASE_URL/v1/objects/presign-upload" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_name": "bucket-a",
    "object_key": "images/cat.jpg",
    "content_type": "image/jpeg",
    "expires_in_seconds": 60
  }'
```

Success (`200`):

```json
{
  "data": {
    "method": "PUT",
    "url": "https://bucket-a.s3.amazonaws.com/...",
    "expires_in_seconds": 60
  }
}
```

### 9) Presign Download URL

`POST /v1/objects/presign-download`

```bash
curl -s -X POST "$BASE_URL/v1/objects/presign-download" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_name": "bucket-a",
    "object_key": "images/cat.jpg",
    "expires_in_seconds": 120
  }'
```

Success (`200`):

```json
{
  "data": {
    "method": "GET",
    "url": "https://bucket-a.s3.amazonaws.com/...",
    "expires_in_seconds": 120
  }
}
```

### 10) List Images

`GET /v1/images`

Two modes:

1. Discovery mode (no `ids` query): returns authorized images with metadata.
2. Resolve mode (with `ids`): resolves provided IDs only.

Discovery mode:

```bash
curl -s "$BASE_URL/v1/images" \
  -H "Authorization: Bearer $TOKEN"
```

Example success (`200`):

```json
{
  "data": {
    "images": [
      {
        "id": "YnVja2V0LWE6aW1hZ2VzL2NhdC5qcGc",
        "bucket_name": "bucket-a",
        "object_key": "images/cat.jpg",
        "size_bytes": 12345,
        "etag": "\"abc123\"",
        "last_modified": "2026-04-12T03:14:15Z",
        "url": "/v1/images/YnVja2V0LWE6aW1hZ2VzL2NhdC5qcGc"
      }
    ]
  }
}
```

Resolve mode:

```bash
curl -s "$BASE_URL/v1/images?ids=YnVja2V0LWE6aW1hZ2VzL2NhdC5qcGc,YnVja2V0LWI6aW1hZ2VzL2RvZy5qcGc" \
  -H "Authorization: Bearer $TOKEN"
```

Example success (`200`):

```json
{
  "data": {
    "images": [
      {
        "id": "YnVja2V0LWE6aW1hZ2VzL2NhdC5qcGc",
        "bucket_name": "bucket-a",
        "object_key": "images/cat.jpg",
        "url": "/v1/images/YnVja2V0LWE6aW1hZ2VzL2NhdC5qcGc"
      },
      {
        "id": "YnVja2V0LWI6aW1hZ2VzL2RvZy5qcGc",
        "bucket_name": "bucket-b",
        "object_key": "images/dog.jpg",
        "url": "/v1/images/YnVja2V0LWI6aW1hZ2VzL2RvZy5qcGc"
      }
    ]
  }
}
```

### 11) Get Image Bytes

`GET /v1/images/{id}`

```bash
ID="YnVja2V0LWE6aW1hZ2VzL2NhdC5qcGc"
curl -i "$BASE_URL/v1/images/$ID" \
  -H "Authorization: Bearer $TOKEN" \
  --output cat.jpg
```

Typical response headers:

- `Content-Type`
- `Content-Length`
- `ETag`

Example success (`200`) headers:

```http
HTTP/1.1 200 OK
Content-Type: image/jpeg
Content-Length: 12345
ETag: "abc123"
```

Body is the raw image bytes.

If you want headers only and no file output:

```bash
curl -s -D - "$BASE_URL/v1/images/$ID" \
  -H "Authorization: Bearer $TOKEN" \
  -o /dev/null
```

### 12) Delete Image by ID

`DELETE /v1/images/{id}`

```bash
ID="YnVja2V0LWE6aW1hZ2VzL2NhdC5qcGc"
curl -s -X DELETE "$BASE_URL/v1/images/$ID" \
  -H "Authorization: Bearer $TOKEN"
```

Success (`200`):

```json
{
  "data": {
    "deleted": true,
    "bucket": "bucket-a",
    "object_key": "images/cat.jpg"
  }
}
```

## Common Failure Examples

Unauthorized (`401`):

```json
{
  "error": {
    "code": "auth_failed",
    "message": "missing bearer token",
    "requestId": "...",
    "details": {
      "reason": "missing"
    }
  }
}
```

Forbidden (`403`):

```json
{
  "error": {
    "code": "forbidden",
    "message": "operation not permitted for this scope",
    "requestId": "...",
    "details": {
      "reason": "prefix_scope"
    }
  }
}
```

Throttle (`429`):

```json
{
  "error": {
    "code": "throttle",
    "message": "rate limit exceeded",
    "requestId": "...",
    "details": {
      "retryAfter": 30,
      "limit": 60,
      "remaining": 0
    }
  }
}
```

Upstream failure (`502`):

```json
{
  "error": {
    "code": "upstream_failure",
    "message": "failed to read image from storage provider",
    "requestId": "..."
  }
}
```
