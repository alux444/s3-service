# Production Droplet Setup (API + Separate Postgres)

This runbook sets up the service on a single production droplet with:
- one dedicated Postgres container
- one API container
- an existing host-level Caddy reverse proxy for HTTPS

It is written for Ubuntu 24.04 on DigitalOcean, but works on any Linux host with Docker.

## 0) Before touching the droplet: prepare AWS runtime credentials

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

## 3) Create isolated Docker network and volume

```bash
docker network create s3-service-net || true
docker volume create s3_service_pgdata || true
```

## 4) Start a separate Postgres instance

Create a Postgres env file:

```bash
mkdir -p /opt/s3-service/runtime
cat > /opt/s3-service/runtime/postgres.env <<'EOF'
POSTGRES_USER=s3_service
POSTGRES_PASSWORD=CHANGE_ME_STRONG_PASSWORD
POSTGRES_DB=s3_service
EOF
chmod 600 /opt/s3-service/runtime/postgres.env
```

Run dedicated Postgres container (separate instance):

```bash
docker run -d \
  --name s3-service-postgres \
  --restart unless-stopped \
  --network s3-service-net \
  -p 127.0.0.1:55432:5432 \
  --env-file /opt/s3-service/runtime/postgres.env \
  -v s3_service_pgdata:/var/lib/postgresql/data \
  --health-cmd='pg_isready -U $$POSTGRES_USER -d $$POSTGRES_DB' \
  --health-interval=5s \
  --health-timeout=3s \
  --health-retries=20 \
  postgres:18-alpine
```

Confirm DB is healthy:

```bash
docker ps --filter name=s3-service-postgres
docker inspect --format='{{json .State.Health}}' s3-service-postgres
```

## 5) Create API runtime env file

Important: inside the production image, migrations are at /app/db/migrations.

```bash
cat > /opt/s3-service/runtime/api.env <<'EOF'
PORT=8080
LOG_LEVEL=info

DATABASE_URL=postgres://s3_service:CHANGE_ME_STRONG_PASSWORD@s3-service-postgres:5432/s3_service?sslmode=disable
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

## 6) Build and run the API container

```bash
cd /opt/s3-service/app
docker build -f Dockerfile -t s3-service-api:prod .

docker run -d \
  --name s3-service-api \
  --restart unless-stopped \
  --network s3-service-net \
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

## 7) Wire into existing Caddy (host install)

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

## 8) First-time data bootstrap (required for real use)

The API starts with schema only. You still need:
1. Bucket connections in bucket_connections.
2. Access policies in access_policies.

Use API calls from docs/003-api-reference.md:
- POST /v1/bucket-connections
- POST /v1/access-policies
- Then perform actions with a valid JWT principal that has matching policy rows.

If your JWT claims do not map to database policy rows, /v1 actions will return forbidden.

## 9) Operations runbook

Restart only API:

```bash
docker restart s3-service-api
```

Restart only Postgres:

```bash
docker restart s3-service-postgres
```

Tail logs:

```bash
docker logs -f s3-service-api
docker logs -f s3-service-postgres
```

## 10) Deploy updates

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
  --network s3-service-net \
  -p 127.0.0.1:8080:8080 \
  --env-file /opt/s3-service/runtime/api.env \
  s3-service-api:prod

curl -sS http://127.0.0.1:8080/health
```

## 11) Backups for the separate Postgres instance

Nightly backup example:

```bash
mkdir -p /opt/s3-service/backups
TS=$(date +%Y%m%d-%H%M%S)

docker exec s3-service-postgres \
  pg_dump -U s3_service s3_service \
  | gzip > /opt/s3-service/backups/s3_service-${TS}.sql.gz
```

Restore example:

```bash
gunzip -c /opt/s3-service/backups/<FILE>.sql.gz \
  | docker exec -i s3-service-postgres psql -U s3_service -d s3_service
```

## 12) Production checklist

- JWT_ENABLED is true.
- DB_MIGRATIONS_DIR is /app/db/migrations.
- Postgres password was changed from placeholder.
- AWS runtime key belongs to droplet-runtime (not admin user).
- API is reachable only through HTTPS entrypoint.
- Backups are running and tested.
