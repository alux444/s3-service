# End-to-End App Onboarding (AWS + Auth0 + S3 Service)

Use this runbook when you have AWS admin access and Auth0 admin access and want to onboard an app so it can use this service to interact with one S3 bucket.

This guide covers:
- AWS resources (bucket, IAM user, assume-role)
- Service runtime credentials
- Auth0 token issuance with required claims
- API bootstrap (bucket connection + access policy)
- App integration patterns for upload/read/delete/presign

## 0) Target architecture

Your app never talks to S3 directly with long-lived AWS credentials.

Flow:
1. App gets an Auth0 access token for this API.
2. App calls this service with Bearer token.
3. Service validates JWT and checks DB authorization policy.
4. Service assumes AWS role and performs S3 operation.

## 1) Prerequisites

- AWS CLI v2
- `jq`
- `curl`
- Access to this repo and running API deployment
- Auth0 tenant admin access

From repo root:

```bash
cd /path/to/s3-service
```

## 2) AWS bootstrap (admin side)

Do not use AWS root user credentials for daily operations. Use an admin IAM identity/profile.

### 2.1 Login as admin profile

```bash
aws configure sso --profile s3-service-admin
aws sso login --profile s3-service-admin
export AWS_PROFILE=s3-service-admin
export AWS_REGION=ap-southeast-2
aws sts get-caller-identity
```

### 2.2 Create/update AWS resources with script

```bash
./scripts/setup-aws-from-admin.sh
```

This creates/updates:
- IAM user: `droplet-runtime`
- S3 bucket baseline (private + BucketOwnerEnforced)
- App assume role: `<project-prefix>-bucket-access`
- Integration role: `<project-prefix>-it-role`

### 2.3 Create runtime access key (manual step)

```bash
AWS_PROFILE=s3-service-admin aws iam create-access-key --user-name droplet-runtime
```

Save securely:
- `AccessKeyId`
- `SecretAccessKey`

You will place these in service runtime env as:
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`

## 3) Configure S3 service runtime

If you already deployed production, align with [docs/005-production-droplet-setup.md](docs/005-production-droplet-setup.md).

Minimum required env in API runtime:

```dotenv
AWS_REGION=ap-southeast-2
AWS_ACCESS_KEY_ID=<droplet_runtime_access_key_id>
AWS_SECRET_ACCESS_KEY=<droplet_runtime_secret_access_key>

JWT_ENABLED=true
JWT_ISSUER=https://<your-auth0-domain>/
JWT_AUDIENCE=<your-auth0-api-identifier>
JWT_JWKS_URL=https://<your-auth0-domain>/.well-known/jwks.json
```

Restart API after env changes.

## 4) Auth0 setup (issuer, audience, claims)

If not done yet, follow [docs/004-auth0-jwt-setup.md](docs/004-auth0-jwt-setup.md).

The access token must include claims used by this service:
- `sub`
- `app_id`
- `project_id`
- `role`
- `principal_type`

### 4.1 Create/verify Auth0 API

- Auth0 Dashboard -> Applications -> APIs
- API Identifier becomes `JWT_AUDIENCE`
- Signing algorithm must be RS256

### 4.2 Create two M2M clients (recommended)

Create two Auth0 Machine-to-Machine applications for clean separation:

1. Bootstrap admin client:
- Used for setup endpoints (`/v1/bucket-connections`, `/v1/access-policies`)
- Claims should include `role=admin`, `principal_type=service`

2. App runtime client:
- Used by your app in normal operation
- Claims usually `role=project-client`, `principal_type=service`

Set `app_id` and `project_id` to the same scope for both clients.

## 5) Get tokens for bootstrap and app runtime

Set shared values:

```bash
export AUTH0_DOMAIN="<your-tenant>.us.auth0.com"
export AUTH0_AUDIENCE="<your-api-identifier>"
```

Get bootstrap admin token:

```bash
export AUTH0_ADMIN_CLIENT_ID="<admin_m2m_client_id>"
export AUTH0_ADMIN_CLIENT_SECRET="<admin_m2m_client_secret>"

ADMIN_TOKEN=$(curl -fsSL "https://${AUTH0_DOMAIN}/oauth/token" \
  -H 'content-type: application/json' \
  -d "{\"client_id\":\"${AUTH0_ADMIN_CLIENT_ID}\",\"client_secret\":\"${AUTH0_ADMIN_CLIENT_SECRET}\",\"audience\":\"${AUTH0_AUDIENCE}\",\"grant_type\":\"client_credentials\"}" \
  | jq -r '.access_token')
```

Get app runtime token:

```bash
export AUTH0_APP_CLIENT_ID="<app_m2m_client_id>"
export AUTH0_APP_CLIENT_SECRET="<app_m2m_client_secret>"

APP_TOKEN=$(curl -fsSL "https://${AUTH0_DOMAIN}/oauth/token" \
  -H 'content-type: application/json' \
  -d "{\"client_id\":\"${AUTH0_APP_CLIENT_ID}\",\"client_secret\":\"${AUTH0_APP_CLIENT_SECRET}\",\"audience\":\"${AUTH0_AUDIENCE}\",\"grant_type\":\"client_credentials\"}" \
  | jq -r '.access_token')
```

## 6) Verify auth scope from service

```bash
export BASE_URL="https://api.yourdomain.com"

curl -sS "${BASE_URL}/v1/auth-check" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" | jq .

curl -sS "${BASE_URL}/v1/auth-check" \
  -H "Authorization: Bearer ${APP_TOKEN}" | jq .
```

Confirm these claims are correct and consistent with your intended scope:
- `project_id`
- `app_id`
- `sub`
- `principal_type`
- `role`

## 7) Register bucket connection (admin token)

Use admin token to connect bucket to app scope.

```bash
export BUCKET_NAME="<your-bucket-name>"
export AWS_ROLE_ARN="arn:aws:iam::<account-id>:role/<project-prefix>-bucket-access"

curl -sS -X POST "${BASE_URL}/v1/bucket-connections" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{\
    \"bucket_name\": \"${BUCKET_NAME}\",\
    \"region\": \"${AWS_REGION}\",\
    \"role_arn\": \"${AWS_ROLE_ARN}\",\
    \"allowed_prefixes\": [\"uploads/\", \"images/\"]\
  }" | jq .
```

Expected success:
- HTTP 201
- `{ "data": { "created": true } }`

## 8) Create access policy for app principal (admin token)

Use admin token to upsert policy for the principal that your app uses (`APP_TOKEN` principal).

First, get the runtime principal values:

```bash
APP_SUB=$(curl -sS "${BASE_URL}/v1/auth-check" -H "Authorization: Bearer ${APP_TOKEN}" | jq -r '.data.sub')
APP_PRINCIPAL_TYPE=$(curl -sS "${BASE_URL}/v1/auth-check" -H "Authorization: Bearer ${APP_TOKEN}" | jq -r '.data.principal_type')
```

Upsert policy:

```bash
curl -sS -X POST "${BASE_URL}/v1/access-policies" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{\
    \"bucket_name\": \"${BUCKET_NAME}\",\
    \"principal_type\": \"${APP_PRINCIPAL_TYPE}\",\
    \"principal_id\": \"${APP_SUB}\",\
    \"role\": \"project-client\",\
    \"can_read\": true,\
    \"can_write\": true,\
    \"can_delete\": true,\
    \"can_list\": true,\
    \"prefix_allowlist\": [\"uploads/\", \"images/\"]\
  }" | jq .
```

Expected success:
- HTTP 200
- `{ "data": { "upserted": true } }`

Important:
- `/v1/access-policies` scope (`project_id`, `app_id`) comes from JWT claims, not body fields.
- Effective prefix access is intersection of:
  - bucket connection `allowed_prefixes`
  - access policy `prefix_allowlist`

## 9) Smoke test app token against object APIs

Use `APP_TOKEN` for runtime operations.

### 9.1 Upload

```bash
curl -sS -X POST "${BASE_URL}/v1/objects/upload" \
  -H "Authorization: Bearer ${APP_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_name": "'"${BUCKET_NAME}"'",
    "object_key": "uploads/hello.txt",
    "content_type": "text/plain",
    "content_b64": "aGVsbG8gd29ybGQ="
  }' | jq .
```

### 9.2 Presign upload

```bash
curl -sS -X POST "${BASE_URL}/v1/objects/presign-upload" \
  -H "Authorization: Bearer ${APP_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_name": "'"${BUCKET_NAME}"'",
    "object_key": "uploads/from-presign.txt",
    "content_type": "text/plain",
    "expires_in_seconds": 60
  }' | jq .
```

### 9.3 List images (if stored under allowed image prefixes)

```bash
curl -sS "${BASE_URL}/v1/images" \
  -H "Authorization: Bearer ${APP_TOKEN}" | jq .
```

## 10) How to wire this into your app

Recommended: app backend calls this service server-to-server.

### 10.1 Runtime sequence

1. App backend obtains Auth0 client credentials token (`APP_TOKEN`).
2. Cache token until near expiry.
3. Call this service endpoints with `Authorization: Bearer <APP_TOKEN>`.
4. Use object keys that stay inside allowed prefixes.

### 10.2 Example (Node.js server)

```javascript
async function getServiceToken() {
  const res = await fetch(`https://${process.env.AUTH0_DOMAIN}/oauth/token`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      client_id: process.env.AUTH0_APP_CLIENT_ID,
      client_secret: process.env.AUTH0_APP_CLIENT_SECRET,
      audience: process.env.AUTH0_AUDIENCE,
      grant_type: "client_credentials"
    })
  });
  if (!res.ok) throw new Error(`token request failed: ${res.status}`);
  return res.json();
}

async function uploadViaS3Service(token, bucketName, objectKey, base64Body) {
  const res = await fetch(`${process.env.S3_SERVICE_BASE_URL}/v1/objects/upload`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      bucket_name: bucketName,
      object_key: objectKey,
      content_type: "text/plain",
      content_b64: base64Body
    })
  });

  const payload = await res.json();
  if (!res.ok) throw new Error(JSON.stringify(payload));
  return payload;
}
```

## 11) Troubleshooting

### 401 on `/v1/auth-check`

- Issuer/audience/JWKS mismatch
- Token not for this API
- Missing required claims

Use [docs/004-auth0-jwt-setup.md](docs/004-auth0-jwt-setup.md).

### 403 on object endpoints but auth-check is 200

- Missing or mismatched `access_policies` row
- Principal mismatch: token `sub` or `principal_type` differs from policy
- Prefix mismatch between `allowed_prefixes`, `prefix_allowlist`, and object key

Use [docs/006-access-policy-bootstrap.md](docs/006-access-policy-bootstrap.md).

### 404 bucket connection not found when upserting policy

- Bucket not registered in current JWT scope (`project_id`, `app_id`)
- Wrong admin token scope used for bootstrap

### 500 upstream S3 failures

- Runtime AWS key invalid or missing
- Assume-role trust/policy not aligned
- Bucket security baseline not met

Re-run [docs/001-aws-initial-setup.md](docs/001-aws-initial-setup.md) and verify runtime env.

## 12) Quick onboarding checklist

- AWS setup script completed.
- Runtime access key created and set in API env.
- Auth0 API configured (RS256, issuer, audience, JWKS).
- Admin and app tokens both return 200 on `/v1/auth-check`.
- Bucket connection created with admin token.
- Access policy upserted for app principal with admin token.
- App token can upload/read/delete/presign within allowed prefixes.
