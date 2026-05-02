# Access Policy Bootstrap (Endpoint Only)

Use this when protected routes return forbidden because no access_policies row exists yet.

Use your custom UI (or curl) to call `POST /v1/access-policies`.

## 0) Supabase: enable Row Level Security (RLS) and example policies

If you host Postgres in Supabase and have enabled RLS globally, you must add table-level RLS and policies so JWT-scoped requests can read and (where appropriate) modify the rows used by this service.

Recommended steps (run these from the Supabase SQL editor or with `psql` using your `DATABASE_URL`):

Run in Supabase SQL editor, or from your terminal:

~~~bash
psql "${DATABASE_URL}" <<'SQL'
-- paste SQL from the next steps here
SQL
~~~

1. Enable RLS on the tables used by this service:

~~~sql
ALTER TABLE public.bucket_connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.access_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.audit_events ENABLE ROW LEVEL SECURITY;
~~~

2. Ensure the `authenticated` role has table privileges (RLS still applies):

~~~sql
GRANT SELECT ON public.bucket_connections TO authenticated;
GRANT SELECT ON public.access_policies TO authenticated;
GRANT SELECT ON public.audit_events TO authenticated;

GRANT INSERT, UPDATE, DELETE ON public.bucket_connections TO authenticated;
GRANT INSERT, UPDATE, DELETE ON public.access_policies TO authenticated;
GRANT INSERT ON public.audit_events TO authenticated;
~~~

3. Create a full policy set that matches this project's auth model.
     - project/app scope is enforced with JWT claims.
     - principals can read their own access policies.
     - admin-scoped tokens can manage connections and access policies.
     - audit events can be read per project/app; writes are admin-only (or use `service_role`).

Example policies (adapt to your claim names if different):

~~~sql
-- bucket_connections: read within project+app scope
CREATE POLICY bucket_connections_select_project_app
    ON public.bucket_connections
    FOR SELECT
    TO authenticated
    USING (
        (select auth.jwt() ->> 'project_id') = project_id
        AND (select auth.jwt() ->> 'app_id') = app_id
        AND is_active = true
    );

-- bucket_connections: admin can insert/update/delete within project+app scope
CREATE POLICY bucket_connections_admin_write
    ON public.bucket_connections
    FOR ALL
    TO authenticated
    USING (
        (select auth.jwt() ->> 'role') = 'admin'
        AND (select auth.jwt() ->> 'project_id') = project_id
        AND (select auth.jwt() ->> 'app_id') = app_id
    )
    WITH CHECK (
        (select auth.jwt() ->> 'role') = 'admin'
        AND (select auth.jwt() ->> 'project_id') = project_id
        AND (select auth.jwt() ->> 'app_id') = app_id
    );

-- access_policies: principal can read their own policies scoped by connection
CREATE POLICY access_policies_select_owner
    ON public.access_policies
    FOR SELECT
    TO authenticated
    USING (
        principal_type = (select auth.jwt() ->> 'principal_type')
        AND principal_id = (select auth.jwt() ->> 'sub')
        AND bucket_connection_id IN (
            select id from public.bucket_connections bc
            where bc.project_id = (select auth.jwt() ->> 'project_id')
                and bc.app_id   = (select auth.jwt() ->> 'app_id')
                and bc.is_active = true
        )
    );

-- access_policies: admin can insert/update/delete within project+app scope
CREATE POLICY access_policies_admin_write
    ON public.access_policies
    FOR ALL
    TO authenticated
    USING (
        (select auth.jwt() ->> 'role') = 'admin'
        AND bucket_connection_id IN (
            select id from public.bucket_connections bc
            where bc.project_id = (select auth.jwt() ->> 'project_id')
                and bc.app_id   = (select auth.jwt() ->> 'app_id')
                and bc.is_active = true
        )
    )
    WITH CHECK (
        (select auth.jwt() ->> 'role') = 'admin'
        AND bucket_connection_id IN (
            select id from public.bucket_connections bc
            where bc.project_id = (select auth.jwt() ->> 'project_id')
                and bc.app_id   = (select auth.jwt() ->> 'app_id')
                and bc.is_active = true
        )
    );

-- audit_events: read within project+app scope
CREATE POLICY audit_events_select_project_app
    ON public.audit_events
    FOR SELECT
    TO authenticated
    USING (
        (select auth.jwt() ->> 'project_id') = project_id
        AND (select auth.jwt() ->> 'app_id') = app_id
    );

-- audit_events: admin-only inserts (or use service_role bypass)
CREATE POLICY audit_events_admin_insert
    ON public.audit_events
    FOR INSERT
    TO authenticated
    WITH CHECK (
        (select auth.jwt() ->> 'role') = 'admin'
        AND (select auth.jwt() ->> 'project_id') = project_id
        AND (select auth.jwt() ->> 'app_id') = app_id
    );
~~~

Notes:
- These examples use `auth.jwt()` helper functions and string-typed claim keys (`project_id`, `app_id`, `sub`, `principal_type`, `role`) as used elsewhere in this repo. If your identity provider uses different claim names, adjust the `->>` keys accordingly.
- The Supabase `service_role` (or any Postgres role with `bypassrls`) can be used for migrations and background jobs. Do NOT use service keys from browser clients.
- If you do not want non-admins to read `audit_events`, remove the SELECT policy and rely on admin or service_role only.
- Index the columns used by policies (`project_id`, `app_id`, `principal_type`, `principal_id`) — the migrations already add useful indexes for `bucket_connections` and `access_policies`.

If you prefer to keep RLS off for a short testing window, do so carefully and only in non-production projects.


## 1) What the endpoint does

It finds the active bucket connection by:
- project_id
- app_id
- bucket_name

Then it inserts or updates one row in access_policies for:
- principal_type
- principal_id

This endpoint is safe to rerun.

## 2) Required values

The endpoint scope is derived from the JWT, not request body fields:
- PROJECT_ID: the project/tenant scope for this bucket connection.
    It must match the project_id claim in the JWT that calls this endpoint.
- APP_ID: the app/client scope inside that project.
    It must match the app_id claim in the JWT that calls this endpoint.

These are the required request payload fields:
- BUCKET_NAME: bucket already registered in bucket_connections
- PRINCIPAL_TYPE: user or service
- PRINCIPAL_ID: must match token sub claim
- ROLE: admin, project-client, or read-only-client

Exact steps to get PROJECT_ID and APP_ID (copy/paste):

1. Use the same token you will send to protected routes.

~~~bash
export BASE_URL="https://api.yourdomain.com"
export TOKEN="PASTE_ACCESS_TOKEN"
~~~

2. Call auth-check and print the claims used by this service.

~~~bash
curl -sS "${BASE_URL}/v1/auth-check" \
    -H "Authorization: Bearer ${TOKEN}" | jq .
~~~

3. Copy values from response data:
- data.project_id -> PROJECT_ID
- data.app_id -> APP_ID
- data.sub -> PRINCIPAL_ID
- data.principal_type -> PRINCIPAL_TYPE

4. Export them directly:

~~~bash
export PROJECT_ID="<data.project_id>"
export APP_ID="<data.app_id>"
export PRINCIPAL_ID="<data.sub>"
export PRINCIPAL_TYPE="<data.principal_type>"
~~~

5. Use those exact values to choose a matching bucket and principal.
    Do not send PROJECT_ID or APP_ID in the request body; they come from JWT claims.

If you cannot call auth-check yet, decode the token locally:

~~~bash
TOKEN="PASTE_ACCESS_TOKEN" python3 - <<'PY'
import base64, json, os

tok = os.environ["TOKEN"]
parts = tok.split('.')
payload = parts[1] + '=' * (-len(parts[1]) % 4)
obj = json.loads(base64.urlsafe_b64decode(payload.encode()))

print("project_id:", obj.get("project_id"))
print("app_id:", obj.get("app_id"))
print("sub:", obj.get("sub"))
print("principal_type:", obj.get("principal_type"))
PY
~~~

Important:
- Do not invent PROJECT_ID or APP_ID.
- They must exactly match the JWT claims in the token being used.
- They must also match the scope under which the bucket connection was created.
- If these values differ, protected object routes return forbidden.

Optional endpoint fields:
- can_read (default true)
- can_write (default false)
- can_delete (default false)
- can_list (default true)
- prefix_allowlist (default empty)

## 3) Preferred for custom UI: call endpoint

Use the same token from step 2 and call:

~~~bash
curl -sS -X POST "${BASE_URL}/v1/access-policies" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
        "bucket_name": "my-bucket",
        "principal_type": "service",
        "principal_id": "auth0|my-service-client-id",
        "role": "admin",
        "can_read": true,
        "can_write": true,
        "can_delete": true,
        "can_list": true,
        "prefix_allowlist": ["uploads/", "images/"]
    }' | jq .
~~~

Expected success:

~~~json
{
    "data": {
        "upserted": true
    }
}
~~~

## 4) Verify row in Postgres

~~~bash
export DATABASE_URL="postgres://postgres.<project-ref>:<password>@db.<project-ref>.supabase.co:5432/postgres?sslmode=require"

psql "${DATABASE_URL}" -c "
SELECT ap.id, ap.principal_type, ap.principal_id, ap.role,
       ap.can_read, ap.can_write, ap.can_delete, ap.can_list,
       ap.prefix_allowlist,
       bc.project_id, bc.app_id, bc.bucket_name
FROM access_policies ap
JOIN bucket_connections bc ON bc.id = ap.bucket_connection_id
ORDER BY ap.updated_at DESC
LIMIT 20;
"
~~~

## 5) After policy bootstrap

Use the same token for API calls:
- Authorization: Bearer <access_token>

Then test a protected route such as:
- GET /v1/auth-check
- POST /v1/objects/presign-upload

If auth-check is 200 but object routes are still forbidden, verify:
- principal_id equals token sub exactly
- principal_type equals token principal_type exactly
- object key prefix matches both bucket connection and policy allowlist
