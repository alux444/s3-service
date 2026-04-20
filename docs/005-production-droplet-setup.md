# Production Droplet Setup (API + Supabase Postgres)

This runbook sets up the service on a production droplet with:
- one API container
- Supabase-managed Postgres
- an existing host-level Caddy reverse proxy for HTTPS

It is written for Ubuntu 24.04 on DigitalOcean, but works on any Linux host with Docker.

## 0) Before touching the droplet

Prepare these first:
- Supabase project created
- Database password set in Supabase
- Connection string copied (direct or pooler)
- AWS runtime credentials prepared (step below)
- JWT issuer/audience/JWKS values confirmed

From your local machine, run the existing AWS setup flow first:

```bash
cd /path/to/s3-service
./scripts/setup-aws-from-admin.sh
```

Then create the runtime access key manually (required by current workflow):

```bash
AWS_PROFILE=s3-service-admin aws iam create-access-key --user-name droplet-runtime
```

Save the returned values securely. You will need:
- AccessKeyId
- SecretAccessKey

Reference: docs/001-aws-initial-setup.md  
Complete auth setup first: docs/004-auth0-jwt-setup.md

## 1) SSH into droplet and install Docker

```bash
ssh root@<DROPLET_IP>
apt-get update && apt-get upgrade -y
apt-get install -y ca-certificates curl gnupg ufw

install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo $VERSION_CODENAME) stable" \
  > /etc/apt/sources.list.d/docker.list

apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
systemctl enable --now docker
```

Harden network access:

```bash
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable
```

## 2) Clone repo on droplet

```bash
mkdir -p /opt/s3-service
cd /opt/s3-service
git clone <YOUR_REPO_URL> app
cd app
git checkout main
```

## 3) Create runtime env file (Supabase)

Important: inside the production image, migrations are at /app/db/migrations.

Use one Supabase connection style:
- Direct connection: best for migrations and admin operations.
- Pooler connection: best for heavy concurrent app traffic.

```bash
mkdir -p /opt/s3-service/runtime
cat > /opt/s3-service/runtime/api.env <<'EOF'
PORT=8080
LOG_LEVEL=info

# Supabase direct connection (recommended for first deploy)
DATABASE_URL=postgres://postgres.<project-ref>:<password>@db.<project-ref>.supabase.co:5432/postgres?sslmode=require

# Supabase pooler example (use instead of direct URL if preferred)
# DATABASE_URL=postgres://postgres.<project-ref>:<password>@aws-0-<region>.pooler.supabase.com:6543/postgres?sslmode=require&default_query_exec_mode=simple_protocol

DB_MIGRATIONS_DIR=/app/db/migrations
DB_MIGRATE_ON_STARTUP=true
DB_SCHEMA_CHECK_ON_STARTUP=true

JWT_ENABLED=true
JWT_ISSUER=https://YOUR_ISSUER/
JWT_AUDIENCE=YOUR_AUDIENCE
JWT_JWKS_URL=https://YOUR_ISSUER/.well-known/jwks.json

AWS_REGION=ap-southeast-2
AWS_ACCESS_KEY_ID=REPLACE_WITH_DROPLET_RUNTIME_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY=REPLACE_WITH_DROPLET_RUNTIME_SECRET_ACCESS_KEY
EOF
chmod 600 /opt/s3-service/runtime/api.env
```

Notes:
- Keep sslmode=require for Supabase.
- After first successful deploy and migration, consider DB_MIGRATE_ON_STARTUP=false to avoid migration attempts on every restart.

## 4) Build and run the API container

```bash
cd /opt/s3-service/app
docker build -f Dockerfile -t s3-service-api:prod .

docker run -d \
  --name s3-service-api \
  --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  --env-file /opt/s3-service/runtime/api.env \
  s3-service-api:prod
```

Validate startup:

```bash
docker logs --tail 200 s3-service-api
curl -sS http://127.0.0.1:8080/health
```

Expected health response:

```json
{"data":{"status":"ok"}}
```

If startup fails with extension error on first migration, enable `pgcrypto` once in Supabase SQL Editor and restart API.

## 5) Wire into existing Caddy (host install)

Point DNS A record first:
- api.yourdomain.com -> your droplet IP

Add a site block to your existing Caddyfile (commonly /etc/caddy/Caddyfile):

```bash
sudo tee -a /etc/caddy/Caddyfile >/dev/null <<'EOF'
api.yourdomain.com {
  encode gzip
  reverse_proxy 127.0.0.1:8080
}
EOF
```

Validate and reload Caddy:

```bash
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

Verify public endpoint:

```bash
curl -i https://api.yourdomain.com/health
```

If Caddy is managed outside systemd on your droplet, use your existing process manager's reload command instead of `systemctl reload caddy`.

## 6) First-time data bootstrap (required for real use)

The API starts with schema only. You still need:
1. Bucket connections in bucket_connections.
2. Access policies in access_policies.

Use API calls from docs/003-api-reference.md:
- POST /v1/bucket-connections
- POST /v1/access-policies
- Then perform actions with a valid JWT principal that has matching policy rows.

If your JWT claims do not map to database policy rows, /v1 actions will return forbidden.

## 7) Operations runbook

Restart API:

```bash
docker restart s3-service-api
```

Tail API logs:

```bash
docker logs -f s3-service-api
```

Quick DB connectivity check from app container:

```bash
docker exec s3-service-api /bin/sh -lc 'echo "DATABASE_URL is set: ${DATABASE_URL:+yes}"'
```

## 8) Deploy updates

```bash
cd /opt/s3-service/app
git fetch --all
git checkout main
git pull --ff-only

docker build -f Dockerfile -t s3-service-api:prod .
docker rm -f s3-service-api

docker run -d \
  --name s3-service-api \
  --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  --env-file /opt/s3-service/runtime/api.env \
  s3-service-api:prod

curl -sS http://127.0.0.1:8080/health
```

## 9) Backups for Supabase

Recommended:
- Enable Supabase automated backups/PITR in project settings.
- Periodically test restore into a staging project.

Optional ad-hoc logical backup from any machine with Docker:

```bash
TS=$(date +%Y%m%d-%H%M%S)
mkdir -p /opt/s3-service/backups

docker run --rm -e PGPASSWORD='<password>' postgres:18-alpine \
  pg_dump "host=db.<project-ref>.supabase.co port=5432 user=postgres.<project-ref> dbname=postgres sslmode=require" \
  | gzip > /opt/s3-service/backups/s3_service-${TS}.sql.gz
```

## 10) Production checklist

- JWT_ENABLED is true.
- DB_MIGRATIONS_DIR is /app/db/migrations.
- DATABASE_URL points to Supabase and includes sslmode=require.
- AWS runtime key belongs to droplet-runtime (not admin user).
- API is reachable only through HTTPS entrypoint.
- Supabase backup/PITR is enabled and restore was tested.
