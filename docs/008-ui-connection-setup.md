# UI Connection Setup (Create Bucket Connections Safely)

Use this guide to onboard a new app and register a new bucket through your `/ui` admin setup console.

This guide assumes:
1. `/ui` is admin-only.
2. `/ui` performs setup by calling this service's onboarding APIs.
3. Token acquisition/refresh happens first, then setup API calls run.

## 1) Architecture and boundaries

Use one setup pattern in this guide:
1. Admin signs into `/ui`.
2. `/ui` gets a valid admin token.
3. `/ui` calls setup APIs on this service.

Guardrails:
1. Keep `/ui` restricted to trusted admins.
2. Do not expose machine-to-machine client secrets in browser code.
3. If `/ui` is browser-based, use admin user tokens instead of embedded M2M secrets.

## 2) Required token claims for this service

This service requires these JWT claims on `/v1/*` calls:
- `sub`
- `app_id`
- `project_id`
- `role`
- `principal_type`

Without these claims, requests fail authentication/validation.

For setup actions in `/ui`, `role` must be `admin`.

### 2.1 Token scope vs AWS prefix scope (important)

These are different layers:
1. Token scope identifies app ownership context:
   - `project_id`
   - `app_id`
   - `sub`
   - `role`
   - `principal_type`
2. AWS object path scope is configured in API data, not token claims:
   - bucket connection `allowed_prefixes`
   - access policy `prefix_allowlist`

So you do not request a token "for a prefix".
You request a token for an app scope (`project_id` + `app_id`), then configure prefixes during bucket connection/policy setup.

If you need isolation by prefix, use a naming convention such as:
1. `allowed_prefixes = ["uploads/project-1/app-2/"]`
2. matching `prefix_allowlist = ["uploads/project-1/app-2/"]`

## 3) Auth0 setup for multi-app onboarding

### 3.1 Create two machine-to-machine clients per app scope

For each app onboarding flow, use two M2M clients:
1. Bootstrap admin client:
   - used by backend only for setup endpoints
   - metadata role should be `admin`
2. Runtime app client:
   - used by backend for normal runtime calls
   - metadata role should usually be `project-client`

Both clients should have the same `app_id` and `project_id` for that app scope.

### 3.2 Set Auth0 client metadata

In Auth0 Dashboard:
1. Applications -> Applications.
2. Open the Machine to Machine app.
3. Settings -> Application Metadata.

Set metadata JSON like:

```json
{
  "app_id": "app-2",
  "project_id": "project-1",
  "role": "project-client",
  "principal_type": "service"
}
```

For the bootstrap admin client, set `role` to `admin`.

### 3.2.1 How `/ui` gets the right token for the app you are onboarding

When onboarding app `app-2` in `project-1`:
1. Set Auth0 metadata on the client used by `/ui` setup flow:
   - `app_id=app-2`
   - `project_id=project-1`
   - `role=admin` (for setup)
   - `principal_type=service`
2. `/ui` requests a token with that client.
3. `/ui` immediately calls `/v1/auth-check` and verifies claims match:
   - `app_id=app-2`
   - `project_id=project-1`
   - `role=admin`
4. Only then enable Create Connection / Upsert Policy buttons.

If claims do not match expected app/project scope, stop and rotate to the correct client credentials.

### 3.3 Configure Auth0 Credentials Exchange Action

Attach this Action to the Client Credentials flow so claims are generated from metadata:

```javascript
exports.onExecuteCredentialsExchange = async (event, api) => {
  const md = event.client.metadata || {};
  const appId = md.app_id;
  const projectId = md.project_id;
  const role = md.role || "project-client";
  const principalType = md.principal_type || "service";

  if (!appId || !projectId) {
    api.access.deny("invalid_request", "Missing app_id or project_id metadata");
    return;
  }

  api.accessToken.setCustomClaim("app_id", appId);
  api.accessToken.setCustomClaim("project_id", projectId);
  api.accessToken.setCustomClaim("role", role);
  api.accessToken.setCustomClaim("principal_type", principalType);
};
```

### 3.4 Authorize each M2M client for your API audience

In Auth0:
1. APIs -> your API -> Machine to Machine Applications.
2. Authorize both the bootstrap admin client and runtime app client.

## 4) Create AWS bucket and gather UI field values

Do this once per new app bucket before opening the onboarding UI.

### 4.1 Use the project setup script (recommended)

From repo root, run:

```bash
aws configure sso --profile s3-service-admin
aws sso login --profile s3-service-admin

AWS_PROFILE=s3-service-admin \
AWS_REGION=ap-southeast-2 \
PROJECT_PREFIX=s3-service \
./scripts/setup-aws-from-admin.sh
```

This creates or updates:
1. bucket with private baseline controls
2. assume-role used by this service for app operations
3. runtime IAM bootstrap user and policies

### 4.2 Derive the exact values for UI onboarding fields

Use these commands to get values:

```bash
export AWS_PROFILE=s3-service-admin
export AWS_REGION=ap-southeast-2
export ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
export PROJECT_PREFIX=s3-service

export BUCKET_NAME=${PROJECT_PREFIX}-data-${ACCOUNT_ID}
export ROLE_NAME=${PROJECT_PREFIX}-bucket-access
export ROLE_ARN=arn:aws:iam::${ACCOUNT_ID}:role/${ROLE_NAME}

echo "bucket_name=${BUCKET_NAME}"
echo "region=${AWS_REGION}"
echo "role_arn=${ROLE_ARN}"
```

### 4.3 Map command values to UI fields

Use these values in your onboarding form:
1. project_id: from app scope design (for example project-1)
2. app_id: from app scope design (for example app-2)
3. bucket_name: value from BUCKET_NAME
4. region: value from AWS_REGION
5. role_arn: value from ROLE_ARN
6. allowed_prefixes: your global guardrails (for example uploads/, images/)

### 4.4 Optional manual checks before UI onboarding

```bash
aws s3api head-bucket --bucket "${BUCKET_NAME}"
aws s3api get-public-access-block --bucket "${BUCKET_NAME}"
aws s3api get-bucket-ownership-controls --bucket "${BUCKET_NAME}"
aws iam get-role --role-name "${ROLE_NAME}"
```

If these succeed, the bucket and role inputs are ready for the app onboarding UI.

## 5) Registering a new bucket (full /ui flow)

This section is written as an operator runbook. Follow it exactly in order.

### 5.1 Pre-flight values you must have before opening `/ui`

Collect these values first:
1. `project_id` (example: `project-1`)
2. `app_id` (example: `app-2`)
3. `bucket_name` (example output from section 4.2)
4. `region` (example: `ap-southeast-2`)
5. `role_arn` (example: `arn:aws:iam::123456789012:role/s3-service-bucket-access`)
6. `allowed_prefixes` for the connection (example: `uploads/project-1/app-2/`)
7. `prefix_allowlist` for the runtime policy (usually same as `allowed_prefixes`)

### 5.2 Token step (must pass before any setup action)

1. Sign in as an admin user in `/ui`, or paste an admin bearer token if your setup page is token-based.
2. Click **Validate Token**.
3. Confirm token check output shows:
   - `role = admin`
   - `project_id = <your target project_id>`
   - `app_id = <your target app_id>`
   - `principal_type` present
4. If any value does not match, stop. Do not continue onboarding with that token.

#### 5.2.1 If your setup page has a manual token field

Generate an admin token (server-side/terminal), then paste it into the UI.

Example token mint command (admin setup client):

```bash
export AUTH0_DOMAIN="<your-tenant>.us.auth0.com"
export AUTH0_CLIENT_ID="<bootstrap-admin-client-id>"
export AUTH0_CLIENT_SECRET="<bootstrap-admin-client-secret>"
export AUTH0_AUDIENCE="<your-api-audience>"

TOKEN=$(curl -fsSL "https://${AUTH0_DOMAIN}/oauth/token" \
   -H 'content-type: application/json' \
   -d "{\"client_id\":\"${AUTH0_CLIENT_ID}\",\"client_secret\":\"${AUTH0_CLIENT_SECRET}\",\"audience\":\"${AUTH0_AUDIENCE}\",\"grant_type\":\"client_credentials\"}" \
   | jq -r '.access_token')

echo "$TOKEN"
```

Important:
1. Treat this token as sensitive.
2. Do not store it permanently in local storage.
3. Re-mint and re-validate when expired.

### 5.3 Fill the Bucket Connection form 

In the Bucket Connection screen, fill exactly these UI fields:

1. **Bucket Name**: exact bucket name from AWS output.
2. **Region**: exact AWS region for that bucket.
3. **Role ARN**: exact IAM role ARN used by this service.
4. **External ID**: optional value required only if the IAM role trust policy enforces an external ID.
5. **Allowed Prefixes**: one prefix per line (or comma separated if your UI uses commas).

Notes:
1. `External ID` can be left blank if your role trust policy does not require it. This is usually for a third party to access, so we don't usually need this.
2. If your security team requires `External ID`, paste the exact value configured in the IAM role trust relationship.
3. Do not add wildcard-style broad prefixes unless explicitly required.

Recommended first-time safe value:
1. `allowed_prefixes = ["uploads/project-1/app-2/"]`

### 5.4 Click Create Connection and verify result

1. Click **Create Connection**.
2. `/ui` sends `POST /v1/bucket-connections` with your admin token.
3. Success criteria in UI:
   - HTTP status indicates success
   - confirmation toast/message shows connection created
4. Failure criteria in UI:
   - show exact API error code/message
   - keep form values so operator can correct and retry

### 5.5 Fill the Access Policy form (runtime principal permissions)

After connection creation, open **Access Policy** for the same `project_id` + `app_id`.

Fill these fields:
1. `bucket_name`: same exact value used in connection step
2. `principal_type`: `service` (for M2M runtime) or `user` (for user-token runtime)
3. `principal_id`: runtime subject ID (usually Auth0 `sub`)
4. `role`: usually `project-client` for runtime services
5. Permission toggles:
   - `can_read = true` (typical)
   - `can_write = true` if uploads are needed
   - `can_delete = false` unless deletes are needed
   - `can_list = true` if listing images/objects is needed
6. `prefix_allowlist`: match least-privilege runtime paths

Recommended first-time safe value:
1. `prefix_allowlist = ["uploads/project-1/app-2/"]`

Important:
1. `POST /v1/access-policies` is admin-only.
2. Effective allowed prefix set is intersection of:
   - bucket connection `allowed_prefixes`
   - policy `prefix_allowlist`

### 5.6 Click Save Policy and verify result

1. Click **Save Policy** or **Upsert Policy**.
2. `/ui` sends `POST /v1/access-policies`.
3. Success criteria:
   - policy upsert confirmation shown
   - no auth/validation errors
4. If `404 not_found`, confirm bucket connection exists in the same `project_id` and `app_id`.

### 5.7 Whitelist the bucket with the droplet account

1. login to AWS CLI: `aws sso login --profile s3-service-admin`
2. run `./scripts/whitelist-bucket-access.sh {bucketNameHere}`

### 5.8 Run validation test in UI

Run one smoke test from the UI:
1. presign upload or tiny upload using a key under your configured prefix

Use a test object key that clearly matches prefix, for example:
1. `uploads/project-1/app-2/smoke-test.txt`

UI should display:
1. auth check summary
2. scope summary (`project_id`, `app_id`, principal)
3. test result and returned error details on failure

## 6) Suggested `/ui` screens

1. App Scope screen:
   - fields: `project_id`, `app_id`
   - shows scope preview
2. Bucket Connection screen:
   - fields: `bucket_name`, `region`, `role_arn`, `allowed_prefixes`
3. Access Policy screen:
   - fields: permissions + `prefix_allowlist`
4. Validation screen:
   - shows test status and next steps
5. Token Status banner:
   - shows token freshness and role
   - blocks setup buttons if token is invalid or non-admin

## 7) Secret handling rules

Do not put in `/ui` frontend code:
1. Auth0 client secret
2. M2M access token
3. AWS credentials

Allowed in `/ui` frontend:
1. admin access token used for setup calls
2. public API base URL

Recommended:
1. keep token in memory/session storage with short lifetime
2. do not persist long-lived setup tokens

## 8) Common errors during bucket registration

1. `401 auth_failed`: issuer/audience/JWKS mismatch or token expired.
2. `403 forbidden`: caller is not admin for `/v1/access-policies`.
3. `404 not_found` during policy upsert: bucket connection missing in same scope.
4. `bucket_security_baseline_failed`: target bucket does not meet required baseline.

## 9) Production recommendations

1. Keep onboarding endpoints behind admin UI only.
2. Use separate bootstrap-admin and runtime clients.
3. Rotate Auth0 client secrets on schedule.
4. Log every onboarding action with request ID and actor.
