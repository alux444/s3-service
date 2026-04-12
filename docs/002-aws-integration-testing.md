# AWS Integration Testing (Simple)

This runs real AWS integration tests for `internal/s3/aws_integration_test.go`.

## What It Tests

- STS assume-role works
- Bucket baseline checks work
- Upload, presign, and delete flows work

## Required Environment

- `RUN_AWS_INTEGRATION=true`
- `AWS_IT_BUCKET`
- `AWS_IT_REGION`
- `AWS_IT_ROLE_ARN`
- `AWS_IT_EXTERNAL_ID` (optional)

## Recommended Flow (One Command)

```bash
ADMIN_PROFILE=s3-service-admin \
RUNTIME_PROFILE=droplet-runtime \
AWS_REGION=ap-southeast-2 \
./scripts/run-aws-it-local.sh
```

This runner does setup, waits 20s for IAM propagation, verifies assume-role, then runs the integration test.

## Important Manual Step (Do Not Skip)

Before first runtime test, create the `droplet-runtime` access key manually from your admin profile:

```bash
AWS_PROFILE=s3-service-admin aws iam create-access-key --user-name droplet-runtime
```

Then configure that key on your machine/droplet:

```bash
aws configure --profile droplet-runtime
aws sts get-caller-identity --profile droplet-runtime
```

## Direct Test Runner (If Setup Already Exists)

```bash
AWS_IT_BUCKET="your-bucket" \
AWS_IT_REGION="ap-southeast-2" \
AWS_IT_ROLE_ARN="arn:aws:iam::123456789012:role/s3-service-it-role" \
AWS_IT_EXTERNAL_ID="optional-external-id" \
./scripts/run-aws-integration.sh
```

## Router API Integration Runner (Real AWS Through HTTP Layer)

This runs the router-level AWS integration test (`internal/httpapi/router/api_aws_integration_test.go`) using the same env vars:

```bash
AWS_IT_BUCKET="your-bucket" \
AWS_IT_REGION="ap-southeast-2" \
AWS_IT_ROLE_ARN="arn:aws:iam::123456789012:role/s3-service-it-role" \
AWS_IT_EXTERNAL_ID="optional-external-id" \
./scripts/run-aws-api-integration.sh
```

Notes:

- Router image discovery (`GET /v1/images`) requires `s3:ListBucket` on the integration role for `it/*` prefixes.
- If router integration starts failing with `upstream_failure` on image list, rerun setup to refresh role policies:

```bash
AWS_PROFILE=s3-service-admin AWS_REGION=ap-southeast-2 ./scripts/setup-aws-from-admin.sh
```

## Notes

- Missing env vars cause the test to skip (not fail).
- Keep admin and runtime profiles separate.
- Use `s3-service-admin` for setup and `droplet-runtime` for runtime/test execution.
