#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

ADMIN_PROFILE="${ADMIN_PROFILE:-s3-service-admin}"
RUNTIME_PROFILE="${RUNTIME_PROFILE:-droplet-runtime}"
AWS_REGION="${AWS_REGION:-ap-southeast-2}"
PROJECT_PREFIX="${PROJECT_PREFIX:-s3-service}"
IT_ROLE_NAME="${IT_ROLE_NAME:-${PROJECT_PREFIX}-it-role}"
IT_EXTERNAL_ID="${IT_EXTERNAL_ID:-${PROJECT_PREFIX}-it-external}"
IAM_PROPAGATION_SLEEP_SECONDS="${IAM_PROPAGATION_SLEEP_SECONDS:-20}"

log() {
  local level="$1"
  shift
  printf '%s [%s] %s\n' "$(date +"%Y-%m-%d %H:%M:%S")" "${level}" "$*"
}

fail() {
  log "ERROR" "$*"
  exit 1
}

aws_admin() {
  AWS_PROFILE="${ADMIN_PROFILE}" AWS_REGION="${AWS_REGION}" AWS_PAGER="" aws "$@"
}

aws_runtime() {
  AWS_PROFILE="${RUNTIME_PROFILE}" AWS_REGION="${AWS_REGION}" AWS_PAGER="" aws "$@"
}

check_admin_login() {
  if ! aws_admin sts get-caller-identity >/dev/null 2>&1; then
    fail "Admin profile ${ADMIN_PROFILE} is not authenticated. Run: aws sso login --profile ${ADMIN_PROFILE}"
  fi
}

check_runtime_profile() {
  if ! aws_runtime sts get-caller-identity >/dev/null 2>&1; then
    fail "Runtime profile ${RUNTIME_PROFILE} is not configured or invalid. Configure it first with: aws configure --profile ${RUNTIME_PROFILE}"
  fi
}

log "INFO" "Running local AWS integration flow"
log "INFO" "AdminProfile=${ADMIN_PROFILE} RuntimeProfile=${RUNTIME_PROFILE} Region=${AWS_REGION}"

check_admin_login
log "INFO" "Admin profile is authenticated"

log "INFO" "Running setup script to create/update required AWS resources"
AWS_PROFILE="${ADMIN_PROFILE}" \
AWS_REGION="${AWS_REGION}" \
PROJECT_PREFIX="${PROJECT_PREFIX}" \
IT_ROLE_NAME="${IT_ROLE_NAME}" \
IT_EXTERNAL_ID="${IT_EXTERNAL_ID}" \
VERIFY_ASSUME_ROLE_WITH_CALLER=false \
"${ROOT_DIR}/scripts/setup-aws-from-admin.sh"

check_runtime_profile
log "INFO" "Runtime profile is configured"

ACCOUNT_ID="$(aws_runtime sts get-caller-identity --query Account --output text)"
BUCKET_NAME="${BUCKET_NAME:-${PROJECT_PREFIX}-data-${ACCOUNT_ID}}"
IT_ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${IT_ROLE_NAME}"

log "INFO" "Sleeping ${IAM_PROPAGATION_SLEEP_SECONDS}s for IAM policy propagation"
sleep "${IAM_PROPAGATION_SLEEP_SECONDS}"

log "INFO" "Verifying runtime profile can assume integration role"
aws_runtime sts assume-role \
  --role-arn "${IT_ROLE_ARN}" \
  --role-session-name local-it-check \
  --external-id "${IT_EXTERNAL_ID}" \
  --query 'Credentials.AccessKeyId' \
  --output text >/dev/null

log "INFO" "Running integration test"
AWS_PROFILE="${RUNTIME_PROFILE}" \
RUN_AWS_INTEGRATION=true \
AWS_IT_BUCKET="${BUCKET_NAME}" \
AWS_IT_REGION="${AWS_REGION}" \
AWS_IT_ROLE_ARN="${IT_ROLE_ARN}" \
AWS_IT_EXTERNAL_ID="${IT_EXTERNAL_ID}" \
"${ROOT_DIR}/scripts/run-aws-integration.sh"

log "INFO" "Local AWS integration flow completed successfully"
