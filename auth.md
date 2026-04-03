# Auth System (Simple Explanation)

This service uses two layers of protection:

1. Authentication: "Who are you?"
2. Authorization: "What are you allowed to do in this bucket and object path?"

## 1) Authentication (JWT)

Authentication happens in HTTP middleware before protected routes run.

- File: `internal/httpapi/middleware/auth.go`
- Verifier: `internal/auth/auth.go`

### What the token must contain

The JWT must include these claims:

- `sub`: principal identifier (the caller identity)
- `app_id`: application scope
- `project_id`: project/tenant scope
- `role`: one of `admin`, `project-client`, `read-only-client`
- `principal_type`: `user` or `service` (required)

Validation rules:

- Signature is verified with JWKS (`RS256` only)
- `iss` and `aud` must match configured values
- Expiry is checked (with 60-second leeway)

If valid, claims are stored in request context and used by handlers/services.

## 2) Core Concepts

### Principal string

A principal is the caller identity. In code, this is the JWT `sub` claim.

- Code field: `claims.Subject`
- DB match target: `access_policies.principal_id`

### Principal type

Principal type tells the service what kind of caller this is:

- `user`
- `service`

In code, this is the JWT `principal_type` claim and maps to `claims.PrincipalType`.

In DB policy lookup, the service now matches on both:

- `access_policies.principal_type`
- `access_policies.principal_id`

This prevents accidental overlap when a user and a service share the same ID string.

### Connection (bucket connection)

A connection is a registered bucket integration for a specific project/app.

- Table: `bucket_connections`
- Key scope: (`project_id`, `app_id`, `bucket_name`)
- Must be active: `is_active = true`
- Has global prefix guardrails: `allowed_prefixes` (TEXT[])

Think of this as the top-level container that says:
"This app in this project can use this bucket, under these broad prefixes."

### Access policy

An access policy is per principal, attached to a connection.

- Table: `access_policies`
- Linked by: `bucket_connection_id`
- Principal identity: (`principal_type`, `principal_id`)
- Role: `role`
- Action booleans:
  - `can_read`
  - `can_write`
  - `can_delete`
  - `can_list`
- Principal prefix list: `prefix_allowlist` (TEXT[])

Think of this as:
"For this specific user/service principal, what actions and sub-prefixes are allowed?"

## 3) Effective Authorization Rule

Main decision code:

- Service: `internal/service/action_auth.go`
- Repository query: `internal/database/ownership.go` (`GetEffectiveAuthorizationPolicy`)

The service allows a request only if all checks pass:

1. Input is valid (`project_id`, `app_id`, `sub`, bucket, action)
2. Principal type is present and valid (`principal_type`)
3. A policy exists for the scoped connection + principal identity (`principal_type` + `principal_id`)
4. Requested action is allowed (`can_read/write/delete/list`)
5. Object key is inside effective prefixes

### Effective prefixes (important)

Allowed prefixes are the intersection of:

- Connection-level prefixes: `bucket_connections.allowed_prefixes`
- Principal-level prefixes: `access_policies.prefix_allowlist`

In code, this is `intersectPrefixes(...)`.

Then the object key must start with one of those intersected prefixes.

In simple terms:
"You only get paths that are allowed by BOTH the bucket connection and your personal policy."

## 4) Decision Reasons

When denied, the service returns a reason code:

- `invalid_input`: required fields or action are invalid
- `bucket_scope`: no matching active connection/policy (or repo error)
- `action_scope`: action boolean is false
- `prefix_scope`: object key is outside effective prefixes

Defined in `internal/auth/action_auth.go` and used by `AuthorizationService.Authorize`.

## 5) Database Tables Used for Auth

From migration `db/migrations/0001_init_schema.sql`:

### `bucket_connections`

Used for bucket ownership/scope and connection-level prefixes.

Relevant columns:

- `project_id`, `app_id`, `bucket_name`, `is_active`
- `allowed_prefixes`

### `access_policies`

Used for principal-level permissions and prefixes.

Relevant columns:

- `bucket_connection_id`
- `principal_type`, `principal_id`
- `role`
- `can_read`, `can_write`, `can_delete`, `can_list`
- `prefix_allowlist`

### `audit_events`

Exists for audit trail data model, but current auth decision path shown above does not write to it yet.

## 6) End-to-End Request Flow

1. Request hits `/v1/*` route (JWT middleware is applied).
2. Middleware extracts Bearer token and verifies JWT.
3. Claims (`sub`, `app_id`, `project_id`, `role`, `principal_type`) are put in context.
4. Handler/service reads claims and calls authorization logic.
5. Repository loads effective policy by joining:
   - `bucket_connections` (scoped by `project_id`, `app_id`, `bucket_name`, active)
  - `access_policies` (scoped by `principal_type` and `principal_id`)
6. Service checks action + prefix intersection.
7. Allow or deny with a stable reason code.

### Concrete example: what the request is asking for

Example request intent:

- Caller wants to upload one object to `bucket-a`
- Target object key is `uploads/avatars/user-1.png`
- Requested capability is `write`

Example HTTP shape (simplified):

```http
POST /v1/objects/upload HTTP/1.1
Authorization: Bearer <jwt>
Content-Type: application/json

{
  "bucket_name": "bucket-a",
  "object_key": "uploads/avatars/user-1.png"
}
```

How auth interprets this request:

1. Authentication asks: is this caller real and trusted?
2. Authorization asks: can this exact principal (`principal_type` + `sub`) do `write` on `bucket-a` for key `uploads/avatars/user-1.png`?

How the decision is made, step by step:

1. JWT middleware validates signature, issuer, audience, expiry.
2. Claims are extracted: `sub`, `project_id`, `app_id`, `role`, `principal_type`.
3. Service resolves principal identity:
  - principal type = `principal_type`
  - principal id = `sub`
4. DB lookup finds an active bucket connection for (`project_id`, `app_id`, `bucket_name`).
5. DB lookup finds matching access policy for (`principal_type`, `principal_id`).
6. Service checks action permission:
  - `write` requires `can_write = true`
7. Service checks prefix permission:
  - compute intersection of `bucket_connections.allowed_prefixes` and `access_policies.prefix_allowlist`
  - require `uploads/avatars/user-1.png` to start with one of those intersected prefixes
8. If all checks pass, request is allowed; otherwise denied with a specific reason (`bucket_scope`, `action_scope`, `prefix_scope`, or `invalid_input`).


## 7) What Is Already Protected

- `/v1/auth-check`: confirms authenticated claims
- `/v1/bucket-connections`: lists active buckets for caller scope (`project_id`, `app_id`)

Both are under JWT middleware.

## 8) Mental Model (One-Liner)

JWT proves identity and scope (`sub`, `project_id`, `app_id`, `principal_type`), then DB policy decides if that exact principal can do the requested action on the requested bucket/object prefix.