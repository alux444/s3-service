#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

: "${AWS_IT_BUCKET:?Set AWS_IT_BUCKET to your test bucket name}"
: "${AWS_IT_REGION:?Set AWS_IT_REGION to the bucket region}"
: "${AWS_IT_ROLE_ARN:?Set AWS_IT_ROLE_ARN to the assume-role ARN}"

export RUN_AWS_INTEGRATION="${RUN_AWS_INTEGRATION:-true}"

go test ./internal/httpapi/router -run AWSIntegration -v -count=1 "$@"
