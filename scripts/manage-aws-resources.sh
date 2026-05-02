#!/usr/bin/env bash
set -euo pipefail

AWS_PROFILE="${AWS_PROFILE:-s3-service-admin}"
AWS_REGION="${AWS_REGION:-ap-southeast-2}"
PROJECT_PREFIX="${PROJECT_PREFIX:-s3-service}"
DROPLET_USER_NAME="${DROPLET_USER_NAME:-droplet-runtime}"
APP_ASSUME_ROLE_NAME="${APP_ASSUME_ROLE_NAME:-${PROJECT_PREFIX}-bucket-access}"
IT_ROLE_NAME="${IT_ROLE_NAME:-${PROJECT_PREFIX}-it-role}"

aws_cmd() {
  AWS_PROFILE="${AWS_PROFILE}" AWS_REGION="${AWS_REGION}" AWS_PAGER="" aws "$@"
}

require_aws() {
  if ! command -v aws >/dev/null 2>&1; then
    echo "ERROR: aws CLI not found in PATH." >&2
    exit 1
  fi
}

account_id() {
  aws_cmd sts get-caller-identity --query Account --output text
}

bucket_name() {
  local account
  account="$(account_id)"
  echo "${PROJECT_PREFIX}-data-${account}"
}

print_context() {
  cat <<EOF
AWS_PROFILE=${AWS_PROFILE}
AWS_REGION=${AWS_REGION}
PROJECT_PREFIX=${PROJECT_PREFIX}
DROPLET_USER_NAME=${DROPLET_USER_NAME}
APP_ASSUME_ROLE_NAME=${APP_ASSUME_ROLE_NAME}
IT_ROLE_NAME=${IT_ROLE_NAME}
BUCKET_NAME=$(bucket_name)
EOF
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

list_resources() {
  local bucket
  bucket="$(bucket_name)"

  echo ""
  echo "=== Context ==="
  print_context

  echo ""
  echo "=== Bucket ==="
  if resource_exists_bucket "${bucket}"; then
    echo "Bucket exists: ${bucket}"
  else
    echo "Bucket missing: ${bucket}"
  fi

  echo ""
  echo "=== IAM User ==="
  if resource_exists_user "${DROPLET_USER_NAME}"; then
    aws_cmd iam get-user --user-name "${DROPLET_USER_NAME}" --query 'User.Arn' --output text
    aws_cmd iam list-user-policies --user-name "${DROPLET_USER_NAME}" --output table || true
    aws_cmd iam list-access-keys --user-name "${DROPLET_USER_NAME}" --output table || true
  else
    echo "User missing: ${DROPLET_USER_NAME}"
  fi

  echo ""
  echo "=== IAM Roles ==="
  if resource_exists_role "${APP_ASSUME_ROLE_NAME}"; then
    aws_cmd iam get-role --role-name "${APP_ASSUME_ROLE_NAME}" --query 'Role.Arn' --output text
    aws_cmd iam list-role-policies --role-name "${APP_ASSUME_ROLE_NAME}" --output table || true
  else
    echo "Role missing: ${APP_ASSUME_ROLE_NAME}"
  fi

  if resource_exists_role "${IT_ROLE_NAME}"; then
    aws_cmd iam get-role --role-name "${IT_ROLE_NAME}" --query 'Role.Arn' --output text
    aws_cmd iam list-role-policies --role-name "${IT_ROLE_NAME}" --output table || true
  else
    echo "Role missing: ${IT_ROLE_NAME}"
  fi
}

confirm() {
  local prompt="$1"
  read -r -p "${prompt} [y/N] " ans
  [[ "${ans}" == "y" || "${ans}" == "Y" ]]
}

delete_bucket() {
  local bucket
  bucket="$(bucket_name)"
  if ! resource_exists_bucket "${bucket}"; then
    echo "Bucket not found: ${bucket}"
    return 0
  fi
  if confirm "Delete bucket ${bucket} and ALL objects?"; then
    aws_cmd s3 rb "s3://${bucket}" --force
    echo "Deleted bucket: ${bucket}"
  else
    echo "Skipped bucket delete."
  fi
}

delete_roles() {
  local role
  for role in "${APP_ASSUME_ROLE_NAME}" "${IT_ROLE_NAME}"; do
    if ! resource_exists_role "${role}"; then
      echo "Role not found: ${role}"
      continue
    fi
    if confirm "Delete role ${role} and its inline policies?"; then
      for policy in $(aws_cmd iam list-role-policies --role-name "${role}" --query 'PolicyNames[]' --output text); do
        aws_cmd iam delete-role-policy --role-name "${role}" --policy-name "${policy}"
      done
      aws_cmd iam delete-role --role-name "${role}"
      echo "Deleted role: ${role}"
    else
      echo "Skipped role delete: ${role}"
    fi
  done
}

delete_user() {
  if ! resource_exists_user "${DROPLET_USER_NAME}"; then
    echo "User not found: ${DROPLET_USER_NAME}"
    return 0
  fi
  if confirm "Delete user ${DROPLET_USER_NAME}, access keys, and inline policies?"; then
    for key in $(aws_cmd iam list-access-keys --user-name "${DROPLET_USER_NAME}" --query 'AccessKeyMetadata[].AccessKeyId' --output text); do
      aws_cmd iam delete-access-key --user-name "${DROPLET_USER_NAME}" --access-key-id "${key}"
    done
    for policy in $(aws_cmd iam list-user-policies --user-name "${DROPLET_USER_NAME}" --query 'PolicyNames[]' --output text); do
      aws_cmd iam delete-user-policy --user-name "${DROPLET_USER_NAME}" --policy-name "${policy}"
    done
    aws_cmd iam delete-user --user-name "${DROPLET_USER_NAME}"
    echo "Deleted user: ${DROPLET_USER_NAME}"
  else
    echo "Skipped user delete."
  fi
}

usage() {
  cat <<EOF
Usage:
  AWS_PROFILE=... AWS_REGION=... PROJECT_PREFIX=... \
  ./scripts/manage-aws-resources.sh

Interactive options will list and delete resources created by setup-aws-from-admin.sh.
EOF
}

main() {
  require_aws

  while true; do
    echo ""
    echo "=== Manage AWS Resources ==="
    echo "1) List resources"
    echo "2) Delete bucket"
    echo "3) Delete roles"
    echo "4) Delete user"
    echo "5) Delete ALL (bucket, roles, user)"
    echo "6) Show context"
    echo "7) Exit"

    read -r -p "Select an option: " choice
    case "${choice}" in
      1) list_resources ;;
      2) delete_bucket ;;
      3) delete_roles ;;
      4) delete_user ;;
      5)
        delete_bucket
        delete_roles
        delete_user
        ;;
      6) print_context ;;
      7) exit 0 ;;
      *) echo "Unknown option: ${choice}" ;;
    esac
  done
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

main
