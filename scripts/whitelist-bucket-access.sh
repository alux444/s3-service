#!/bin/bash

# Configure droplet-runtime -> role assume access and whitelist bucket CRUD on the role.
# Usage: ./whitelist-bucket-access.sh <bucket-name> [iam-user] [role-name] [aws-profile]

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <bucket-name> [iam-user] [role-name] [aws-profile]"
  exit 1
fi

BUCKET_NAME="$1"
IAM_USER="${2:-droplet-runtime}"
ROLE_NAME="${3:-locket-bucket-access}"
AWS_PROFILE="${4:-s3-service-admin}"

echo "Whitelisting all CRUD operations for bucket: $BUCKET_NAME"
echo "Using AWS profile: $AWS_PROFILE"
echo "Target IAM user: $IAM_USER"
echo "Target IAM role: $ROLE_NAME"

if ! command -v jq >/dev/null 2>&1; then
  echo "Error: jq is required. Install jq and re-run."
  exit 1
fi

ACCOUNT_ID="$(AWS_PROFILE="$AWS_PROFILE" aws sts get-caller-identity --query Account --output text)"
ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${ROLE_NAME}"
USER_ARN="arn:aws:iam::${ACCOUNT_ID}:user/${IAM_USER}"

echo "Resolved account ID: $ACCOUNT_ID"
echo "Role ARN: $ROLE_ARN"
echo "User ARN: $USER_ARN"

# Create a temp directory for policy documents and clean it up on exit.
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

ASSUME_POLICY_NAME="AllowAssumeRole-${ROLE_NAME}"
ASSUME_POLICY_DOCUMENT_FILE="${TMP_DIR}/allow-assume-role.json"
cat > "$ASSUME_POLICY_DOCUMENT_FILE" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowAssumeTargetRole",
      "Effect": "Allow",
      "Action": "sts:AssumeRole",
      "Resource": "${ROLE_ARN}"
    }
  ]
}
EOF

echo "Applying user assume-role policy..."
AWS_PROFILE="$AWS_PROFILE" aws iam put-user-policy \
  --user-name "$IAM_USER" \
  --policy-name "$ASSUME_POLICY_NAME" \
  --policy-document "file://${ASSUME_POLICY_DOCUMENT_FILE}"

echo "Merging role trust policy to include user..."
CURRENT_TRUST_FILE="${TMP_DIR}/current-trust.json"
UPDATED_TRUST_FILE="${TMP_DIR}/updated-trust.json"
AWS_PROFILE="$AWS_PROFILE" aws iam get-role \
  --role-name "$ROLE_NAME" \
  --query 'Role.AssumeRolePolicyDocument' \
  --output json > "$CURRENT_TRUST_FILE"

jq --arg userArn "$USER_ARN" '
  .Statement = ((.Statement // []) | map(
    if (.Principal.AWS? | type) == "string" and .Principal.AWS != $userArn then
      .Principal.AWS = [ .Principal.AWS, $userArn ]
    elif (.Principal.AWS? | type) == "array" and ((.Principal.AWS | index($userArn)) == null) then
      .Principal.AWS += [ $userArn ]
    else
      .
    end
  ))
  | if any(.Statement[]?; .Action == "sts:AssumeRole" and ((.Principal.AWS == $userArn) or ((.Principal.AWS | type) == "array" and (.Principal.AWS | index($userArn) != null)))) then
      .
    else
      .Statement += [{
        "Sid": "TrustDropletRuntimeUser",
        "Effect": "Allow",
        "Principal": { "AWS": $userArn },
        "Action": "sts:AssumeRole"
      }]
    end
' "$CURRENT_TRUST_FILE" > "$UPDATED_TRUST_FILE"

AWS_PROFILE="$AWS_PROFILE" aws iam update-assume-role-policy \
  --role-name "$ROLE_NAME" \
  --policy-document "file://${UPDATED_TRUST_FILE}"

echo "Applying bucket CRUD policy to role..."
ROLE_POLICY_NAME="BucketAccess-${BUCKET_NAME}"
ROLE_POLICY_DOCUMENT_FILE="${TMP_DIR}/bucket-role-access.json"
cat > "$ROLE_POLICY_DOCUMENT_FILE" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "ListBucket",
      "Effect": "Allow",
      "Action": "s3:ListBucket",
      "Resource": "arn:aws:s3:::${BUCKET_NAME}"
    },
    {
      "Sid": "ObjectCRUD",
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:GetObjectVersion",
        "s3:DeleteObjectVersion"
      ],
      "Resource": "arn:aws:s3:::${BUCKET_NAME}/*"
    }
  ]
}
EOF

AWS_PROFILE="$AWS_PROFILE" aws iam put-role-policy \
  --role-name "$ROLE_NAME" \
  --policy-name "$ROLE_POLICY_NAME" \
  --policy-document "file://${ROLE_POLICY_DOCUMENT_FILE}"

echo "Success! Applied IAM updates:"
echo "  - User policy '$ASSUME_POLICY_NAME' on '$IAM_USER'"
echo "  - Trust update on role '$ROLE_NAME' to allow '$USER_ARN'"
echo "  - Role policy '$ROLE_POLICY_NAME' on '$ROLE_NAME'"
echo ""
echo "Role '$ROLE_NAME' now has these bucket permissions on '$BUCKET_NAME':"
echo "  - s3:ListBucket"
echo "  - s3:GetObject"
echo "  - s3:PutObject"
echo "  - s3:DeleteObject"
echo "  - s3:GetObjectVersion"
echo "  - s3:DeleteObjectVersion"
echo ""
echo "User '$IAM_USER' can now call sts:AssumeRole on '$ROLE_NAME'."
echo "Changes take effect immediately."
