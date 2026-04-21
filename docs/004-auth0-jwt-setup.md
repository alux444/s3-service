# Auth0 Setup for JWT Issuer, Audience, and JWKS

Use this guide before production deployment. It gives exact Auth0 values for:
- JWT_ISSUER
- JWT_AUDIENCE
- JWT_JWKS_URL

This service validates RS256 tokens and requires claim names: sub, app_id, project_id, role, principal_type.

## 1) Create or verify Auth0 API

In Auth0 Dashboard:
1. Go to Applications -> APIs.
2. Create API (or open existing API used by this service).
3. Set Identifier to a stable value, for example https://api.s3-service.
4. Confirm Signing Algorithm is RS256.
5. Access policies: Allow via Client Grant as settings - only personal app scopes

Your API Identifier is the JWT_AUDIENCE value.

## 2) Get issuer and JWKS values from your tenant

Find your Auth0 tenant domain, for example your-tenant.us.auth0.com.

Set:
- JWT_ISSUER=https://YOUR_TENANT_DOMAIN/
- JWT_JWKS_URL=https://YOUR_TENANT_DOMAIN/.well-known/jwks.json

Important: keep the trailing slash in JWT_ISSUER.

Validate OIDC metadata:

```bash
export AUTH0_DOMAIN="YOUR_TENANT_DOMAIN"
curl -fsSL "https://${AUTH0_DOMAIN}/.well-known/openid-configuration" | jq -r '.issuer, .jwks_uri'
```

The output must match JWT_ISSUER and JWT_JWKS_URL.

## 3) Set audience value

Set JWT_AUDIENCE to your Auth0 API Identifier exactly.

Example:

```dotenv
JWT_AUDIENCE=https://api.s3-service
```

## 4) Ensure tokens contain required custom claims

This service requires access token claims:
- app_id
- project_id
- role
- principal_type

If your Auth0 token does not include these, add an Auth0 Action.

For machine-to-machine client credentials flow, create Action in Auth0:
1. Go to Actions -> Library -> Build Custom.
2. Trigger: Credentials Exchange.
3. Add code:

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

- We need to set Settings -> Advanced Settings -> Application Metadata for these values

For user login flow, use Post Login trigger and set principal_type to user.

## 5) Request a real Auth0 access token and inspect it

Go to Applications -> the API. This should be Machine to Machine API.

Decode and check claims:

```bash
TOKEN="$TOKEN" python3 - <<'PY'
import base64, json, os

tok = os.environ["TOKEN"]
parts = tok.split('.')
payload = parts[1] + '=' * (-len(parts[1]) % 4)
obj = json.loads(base64.urlsafe_b64decode(payload.encode()))
print(json.dumps({
    'iss': obj.get('iss'),
    'aud': obj.get('aud'),
    'sub': obj.get('sub'),
    'app_id': obj.get('app_id'),
    'project_id': obj.get('project_id'),
    'role': obj.get('role'),
    'principal_type': obj.get('principal_type')
}, indent=2))
PY
```

## 6) Apply values on droplet

Edit /opt/s3-service/runtime/api.env:

```dotenv
JWT_ENABLED=true
JWT_ISSUER=https://YOUR_TENANT_DOMAIN/
JWT_AUDIENCE=https://api.s3-service
JWT_JWKS_URL=https://YOUR_TENANT_DOMAIN/.well-known/jwks.json
```

Restart service:

```bash
docker restart s3-service-api
docker logs --tail 200 s3-service-api
```

## 7) Verify with live endpoint

```bash
export BASE_URL="https://api.yourdomain.com"
export TOKEN="PASTE_VALID_AUTH0_ACCESS_TOKEN"

curl -i "${BASE_URL}/v1/auth-check" \
  -H "Authorization: Bearer ${TOKEN}"
```

Expected result is HTTP 200 with decoded claims in response data.

## 8) Troubleshooting

401 invalid from /v1/auth-check:
- Issuer mismatch. Check trailing slash in JWT_ISSUER.
- Audience mismatch. Check JWT_AUDIENCE equals Auth0 API Identifier.
- JWKS mismatch. Check tenant domain and /.well-known/jwks.json.
- Wrong token type. Use access token for your API, not ID token.

Service exits during startup mentioning JWT config:
- JWT_ENABLED is true but one of JWT_ISSUER, JWT_AUDIENCE, JWT_JWKS_URL is empty.

403 on /v1 object routes but /v1/auth-check is 200:
- Auth works, but authorization mapping is missing in database.
- Add access_policies rows matching principal_type + principal_id (subject).

## 9) Quick checklist

- Auth0 API uses RS256.
- JWT_ISSUER matches https://TENANT_DOMAIN/.
- JWT_AUDIENCE matches Auth0 API Identifier.
- JWT_JWKS_URL matches https://TENANT_DOMAIN/.well-known/jwks.json.
- Access token contains sub, app_id, project_id, role, principal_type.
