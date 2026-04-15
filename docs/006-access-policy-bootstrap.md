# Access Policy Bootstrap (Endpoint Only)

Use this when protected routes return forbidden because no access_policies row exists yet.

Use your custom UI (or curl) to call `POST /v1/access-policies`.

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
docker exec -i s3-service-postgres psql -U s3_service -d s3_service -c "
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
