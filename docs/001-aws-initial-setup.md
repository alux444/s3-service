# AWS Initial Setup (Simple)

Use this guide to set up AWS for this project from a fresh account with the least amount of manual work.

Default region: `ap-southeast-2`

## 1) Login as Admin (Human)

Why this matters: you need an authenticated admin session to create IAM users/roles/policies.

```bash
aws configure sso --profile s3-service-admin
aws sso login --profile s3-service-admin
export AWS_PROFILE=s3-service-admin
export AWS_REGION=ap-southeast-2
aws sts get-caller-identity
```

## 2) Run Setup Script

Why this matters: this creates/updates bucket + roles + policies consistently.

```bash
./scripts/setup-aws-from-admin.sh
```

## 3) Manually Create Runtime Access Key (Required)

Run this manually from your admin profile:

```bash
AWS_PROFILE=s3-service-admin aws iam create-access-key --user-name droplet-runtime
```

Save the returned `AccessKeyId` and `SecretAccessKey` securely.

## 4) Configure Runtime Profile Locally Or On Droplet

Why this matters: app/runtime commands should use `droplet-runtime`, not your admin login.

```bash
aws configure --profile droplet-runtime
# paste the key pair from step 3
aws sts get-caller-identity --profile droplet-runtime
```

## 5) Run End-to-End Integration Flow

```bash
ADMIN_PROFILE=s3-service-admin \
RUNTIME_PROFILE=droplet-runtime \
AWS_REGION=ap-southeast-2 \
./scripts/run-aws-it-local.sh
```

The runner waits 20s after policy updates to reduce IAM propagation race failures.

## Important Concepts

- `s3-service-admin`: your human admin CLI profile for setup tasks.
- `droplet-runtime`: non-interactive runtime profile used by app/tests.
- App role (`s3-service-bucket-access`): role assumed for S3 operations.
- IT role (`s3-service-it-role`): role assumed by integration tests.
- External ID (`s3-service-it-external`): extra trust check used by the IT role.

## Quick Checks

```bash
AWS_PROFILE=s3-service-admin aws iam get-user --user-name droplet-runtime
AWS_PROFILE=s3-service-admin aws iam get-role --role-name s3-service-bucket-access
AWS_PROFILE=s3-service-admin aws iam get-role --role-name s3-service-it-role
AWS_PROFILE=droplet-runtime aws sts get-caller-identity
```

If all commands succeed, your base setup is good.
