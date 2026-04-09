#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="${ROOT_DIR}/tmp/aws-setup-logs"
mkdir -p "${LOG_DIR}"
LOG_FILE="${LOG_DIR}/setup-$(date +%Y%m%d-%H%M%S).log"
exec > >(tee -a "${LOG_FILE}") 2>&1

AWS_PROFILE="${AWS_PROFILE:-s3-service-admin}"
AWS_REGION="${AWS_REGION:-ap-southeast-2}"
PROJECT_PREFIX="${PROJECT_PREFIX:-s3-service}"
DROPLET_USER_NAME="${DROPLET_USER_NAME:-droplet-runtime}"
APP_ASSUME_ROLE_NAME="${APP_ASSUME_ROLE_NAME:-${PROJECT_PREFIX}-bucket-access}"
IT_ROLE_NAME="${IT_ROLE_NAME:-${PROJECT_PREFIX}-it-role}"
IT_EXTERNAL_ID="${IT_EXTERNAL_ID:-${PROJECT_PREFIX}-it-external}"
CREATE_DROPLET_ACCESS_KEY="${CREATE_DROPLET_ACCESS_KEY:-false}"
VERIFY_ASSUME_ROLE_WITH_CALLER="${VERIFY_ASSUME_ROLE_WITH_CALLER:-false}"

log() {
  local level="$1"
  shift
  printf '%s [%s] %s\n' "$(date +"%Y-%m-%d %H:%M:%S")" "${level}" "$*"
}

fail() {
  log "ERROR" "$*"
  exit 1
}

aws_cmd() {
  AWS_PROFILE="${AWS_PROFILE}" AWS_REGION="${AWS_REGION}" AWS_PAGER="" aws "$@"
}

resource_exists_user() {
  aws_cmd iam get-user --user-name "$1" >/dev/null 2>&1
}

resource_exists_role() {
  aws_cmd iam get-role --role-name "$1" >/dev/null 2>&1
}

resource_exists_bucket() {
  aws_cmd s3api head-bucket --bucket "$1" >/dev/null 2>&1
}

bool_is_true() {
  local val
  val="$(printf '%s' "${1}" | tr '[:upper:]' '[:lower:]')"
  [[ "${val}" == "true" || "${1}" == "1" || "${val}" == "yes" ]]
}

log "INFO" "Starting AWS setup script"
log "INFO" "Profile=${AWS_PROFILE} Region=${AWS_REGION} ProjectPrefix=${PROJECT_PREFIX}"

ACCOUNT_ID="$(aws_cmd sts get-caller-identity --query Account --output text)"
CALLER_ARN="$(aws_cmd sts get-caller-identity --query Arn --output text)"
[[ -n "${ACCOUNT_ID}" ]] || fail "Unable to resolve AWS account ID"

BUCKET_NAME="${BUCKET_NAME:-${PROJECT_PREFIX}-data-${ACCOUNT_ID}}"
APP_ASSUME_ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${APP_ASSUME_ROLE_NAME}"
IT_ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${IT_ROLE_NAME}"
DROPLET_USER_ARN="arn:aws:iam::${ACCOUNT_ID}:user/${DROPLET_USER_NAME}"

log "INFO" "Caller ARN: ${CALLER_ARN}"
log "INFO" "Target bucket: ${BUCKET_NAME}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

# 1) Create or ensure droplet runtime user
if resource_exists_user "${DROPLET_USER_NAME}"; then
  log "INFO" "IAM user exists: ${DROPLET_USER_NAME}"
else
  log "INFO" "Creating IAM user: ${DROPLET_USER_NAME}"
  aws_cmd iam create-user --user-name "${DROPLET_USER_NAME}" >/dev/null
fi

cat > "${TMP_DIR}/droplet-runtime-bootstrap-policy.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowAssumeProjectRoles",
      "Effect": "Allow",
      "Action": "sts:AssumeRole",
      "Resource": [
        "${APP_ASSUME_ROLE_ARN}",
        "${IT_ROLE_ARN}"
      ]
    },
    {
      "Sid": "AllowCallerIdentity",
      "Effect": "Allow",
      "Action": "sts:GetCallerIdentity",
      "Resource": "*"
    }
  ]
}
EOF

log "INFO" "Applying inline bootstrap policy to user ${DROPLET_USER_NAME}"
aws_cmd iam put-user-policy \
  --user-name "${DROPLET_USER_NAME}" \
  --policy-name "${PROJECT_PREFIX}-droplet-bootstrap" \
  --policy-document "file://${TMP_DIR}/droplet-runtime-bootstrap-policy.json" >/dev/null

if bool_is_true "${CREATE_DROPLET_ACCESS_KEY}"; then
  key_count="$(aws_cmd iam list-access-keys --user-name "${DROPLET_USER_NAME}" --query 'length(AccessKeyMetadata)' --output text)"
  if [[ "${key_count}" -ge 2 ]]; then
    fail "User ${DROPLET_USER_NAME} already has 2 access keys. Remove one before creating another."
  fi

  if [[ "${key_count}" -eq 0 ]]; then
    ACCESS_KEY_FILE="${LOG_DIR}/${DROPLET_USER_NAME}-access-key-$(date +%Y%m%d-%H%M%S).json"
    log "WARN" "Creating access key for ${DROPLET_USER_NAME}; save it securely."
    aws_cmd iam create-access-key --user-name "${DROPLET_USER_NAME}" > "${ACCESS_KEY_FILE}"
    chmod 600 "${ACCESS_KEY_FILE}"
    log "WARN" "Access key material written to ${ACCESS_KEY_FILE}"
  else
    log "INFO" "Skipping access key creation; user already has ${key_count} key(s)."
  fi
else
  log "INFO" "CREATE_DROPLET_ACCESS_KEY is false; skipping access key creation."
fi

# 2) Create or ensure bucket baseline
if resource_exists_bucket "${BUCKET_NAME}"; then
  log "INFO" "Bucket exists: ${BUCKET_NAME}"
else
  log "INFO" "Creating bucket: ${BUCKET_NAME}"
  if [[ "${AWS_REGION}" == "us-east-1" ]]; then
    aws_cmd s3api create-bucket --bucket "${BUCKET_NAME}" >/dev/null
  else
    aws_cmd s3api create-bucket \
      --bucket "${BUCKET_NAME}" \
      --region "${AWS_REGION}" \
      --create-bucket-configuration "LocationConstraint=${AWS_REGION}" >/dev/null
  fi
fi

log "INFO" "Applying bucket public access block"
cat > "${TMP_DIR}/public-access-block.json" <<EOF
{
  "BlockPublicAcls": true,
  "IgnorePublicAcls": true,
  "BlockPublicPolicy": true,
  "RestrictPublicBuckets": true
}
EOF

aws_cmd s3api put-public-access-block \
  --bucket "${BUCKET_NAME}" \
  --public-access-block-configuration "file://${TMP_DIR}/public-access-block.json" >/dev/null

log "INFO" "Applying bucket ownership controls"
cat > "${TMP_DIR}/ownership-controls.json" <<EOF
{
  "Rules": [
    {
      "ObjectOwnership": "BucketOwnerEnforced"
    }
  ]
}
EOF

aws_cmd s3api put-bucket-ownership-controls \
  --bucket "${BUCKET_NAME}" \
  --ownership-controls "file://${TMP_DIR}/ownership-controls.json" >/dev/null

# 3) Create or update app assume role
cat > "${TMP_DIR}/app-assume-role-trust.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "${DROPLET_USER_ARN}"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
EOF

if resource_exists_role "${APP_ASSUME_ROLE_NAME}"; then
  log "INFO" "Role exists: ${APP_ASSUME_ROLE_NAME}; updating trust policy"
  aws_cmd iam update-assume-role-policy \
    --role-name "${APP_ASSUME_ROLE_NAME}" \
    --policy-document "file://${TMP_DIR}/app-assume-role-trust.json" >/dev/null
else
  log "INFO" "Creating role: ${APP_ASSUME_ROLE_NAME}"
  aws_cmd iam create-role \
    --role-name "${APP_ASSUME_ROLE_NAME}" \
    --assume-role-policy-document "file://${TMP_DIR}/app-assume-role-trust.json" >/dev/null
fi

cat > "${TMP_DIR}/app-assume-role-policy.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "ObjectCrudOnProjectPrefixes",
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:GetObject",
        "s3:DeleteObject"
      ],
      "Resource": "arn:aws:s3:::${BUCKET_NAME}/uploads/*"
    },
    {
      "Sid": "OptionalListForFutureImageListing",
      "Effect": "Allow",
      "Action": "s3:ListBucket",
      "Resource": "arn:aws:s3:::${BUCKET_NAME}",
      "Condition": {
        "StringLike": {
          "s3:prefix": [
            "uploads/*"
          ]
        }
      }
    },
    {
      "Sid": "BucketSecurityBaselineReads",
      "Effect": "Allow",
      "Action": [
        "s3:GetPublicAccessBlock",
        "s3:GetBucketPolicyStatus",
        "s3:GetBucketOwnershipControls"
      ],
      "Resource": "arn:aws:s3:::${BUCKET_NAME}"
    }
  ]
}
EOF

log "INFO" "Applying inline policy to role ${APP_ASSUME_ROLE_NAME}"
aws_cmd iam put-role-policy \
  --role-name "${APP_ASSUME_ROLE_NAME}" \
  --policy-name "${PROJECT_PREFIX}-bucket-access" \
  --policy-document "file://${TMP_DIR}/app-assume-role-policy.json" >/dev/null

# 4) Create or update integration role
cat > "${TMP_DIR}/it-role-trust.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "${DROPLET_USER_ARN}"
      },
      "Action": "sts:AssumeRole",
      "Condition": {
        "StringEquals": {
          "sts:ExternalId": "${IT_EXTERNAL_ID}"
        }
      }
    }
  ]
}
EOF

if resource_exists_role "${IT_ROLE_NAME}"; then
  log "INFO" "Role exists: ${IT_ROLE_NAME}; updating trust policy"
  aws_cmd iam update-assume-role-policy \
    --role-name "${IT_ROLE_NAME}" \
    --policy-document "file://${TMP_DIR}/it-role-trust.json" >/dev/null
else
  log "INFO" "Creating role: ${IT_ROLE_NAME}"
  aws_cmd iam create-role \
    --role-name "${IT_ROLE_NAME}" \
    --assume-role-policy-document "file://${TMP_DIR}/it-role-trust.json" >/dev/null
fi

cat > "${TMP_DIR}/it-role-policy.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "IntegrationObjectCrud",
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:GetObject",
        "s3:DeleteObject"
      ],
      "Resource": "arn:aws:s3:::${BUCKET_NAME}/it/*"
    },
    {
      "Sid": "IntegrationBaselineReads",
      "Effect": "Allow",
      "Action": [
        "s3:GetPublicAccessBlock",
        "s3:GetBucketPolicyStatus",
        "s3:GetBucketOwnershipControls"
      ],
      "Resource": "arn:aws:s3:::${BUCKET_NAME}"
    }
  ]
}
EOF

log "INFO" "Applying inline policy to role ${IT_ROLE_NAME}"
aws_cmd iam put-role-policy \
  --role-name "${IT_ROLE_NAME}" \
  --policy-name "${PROJECT_PREFIX}-it-access" \
  --policy-document "file://${TMP_DIR}/it-role-policy.json" >/dev/null

# 5) Verification checks
log "INFO" "Running verification checks"
aws_cmd iam get-user --user-name "${DROPLET_USER_NAME}" --query 'User.Arn' --output text >/dev/null
aws_cmd iam get-role --role-name "${APP_ASSUME_ROLE_NAME}" --query 'Role.Arn' --output text >/dev/null
aws_cmd iam get-role --role-name "${IT_ROLE_NAME}" --query 'Role.Arn' --output text >/dev/null
aws_cmd s3api get-public-access-block --bucket "${BUCKET_NAME}" --query 'PublicAccessBlockConfiguration' --output json >/dev/null
aws_cmd s3api get-bucket-ownership-controls --bucket "${BUCKET_NAME}" --query 'OwnershipControls.Rules' --output json >/dev/null

if bool_is_true "${VERIFY_ASSUME_ROLE_WITH_CALLER}"; then
  log "INFO" "VERIFY_ASSUME_ROLE_WITH_CALLER=true, validating sts:AssumeRole with current caller"
  aws_cmd sts assume-role --role-arn "${APP_ASSUME_ROLE_ARN}" --role-session-name app-runtime-check --query 'Credentials.AccessKeyId' --output text >/dev/null
  aws_cmd sts assume-role --role-arn "${IT_ROLE_ARN}" --role-session-name it-runtime-check --external-id "${IT_EXTERNAL_ID}" --query 'Credentials.AccessKeyId' --output text >/dev/null
else
  log "INFO" "Skipping sts:AssumeRole verification with current caller (expected if trust is limited to ${DROPLET_USER_ARN})"
  log "INFO" "To force caller-based assume-role verification, rerun with VERIFY_ASSUME_ROLE_WITH_CALLER=true"
fi

log "INFO" "AWS setup completed successfully"
log "INFO" "Log file: ${LOG_FILE}"

cat <<EOF

=== NEXT STEPS ===

1) Configure droplet runtime profile (on droplet host):
   aws configure --profile droplet-runtime

2) Export integration test variables:
   export AWS_PROFILE=droplet-runtime
   export RUN_AWS_INTEGRATION=true
   export AWS_IT_BUCKET=${BUCKET_NAME}
   export AWS_IT_REGION=${AWS_REGION}
   export AWS_IT_ROLE_ARN=${IT_ROLE_ARN}
   export AWS_IT_EXTERNAL_ID=${IT_EXTERNAL_ID}

3) Run integration test:
   ./scripts/run-aws-integration.sh

=== CREATED/UPDATED RESOURCES ===
Bucket: ${BUCKET_NAME}
Runtime user: ${DROPLET_USER_NAME}
App role: ${APP_ASSUME_ROLE_ARN}
Integration role: ${IT_ROLE_ARN}

EOF
