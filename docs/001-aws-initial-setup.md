# AWS Initial Setup From Fresh Root Account

This guide bootstraps AWS access for this project from a brand-new root account state.

It includes:

- Secure root account baseline
- Human admin access via IAM Identity Center
- Runtime identity for droplet
- Least-privilege IAM policies for current code and planned project scope
- Integration-test role and trust policy

Default region in this project: `ap-southeast-2`

## Automated Setup Script (Recommended)

If you already have `s3-service-admin` Identity Center access, use this script to create/update the remaining AWS resources:

- Script: `scripts/setup-aws-from-admin.sh`
- Logs: `tmp/aws-setup-logs/setup-YYYYMMDD-HHMMSS.log`

### Prerequisites

1. AWS CLI is installed.
2. You can run `aws sts get-caller-identity` with your admin profile.

### Run with defaults

```bash
export AWS_PROFILE=s3-service-admin
export AWS_REGION=ap-southeast-2

./scripts/setup-aws-from-admin.sh
```

### Optional overrides

```bash
AWS_PROFILE=s3-service-admin \
AWS_REGION=ap-southeast-2 \
PROJECT_PREFIX=s3-service \
DROPLET_USER_NAME=droplet-runtime \
APP_ASSUME_ROLE_NAME=s3-service-bucket-access \
IT_ROLE_NAME=s3-service-it-role \
IT_EXTERNAL_ID=s3-service-it-external \
BUCKET_NAME=s3-service-data-<account-id> \
CREATE_DROPLET_ACCESS_KEY=false \
./scripts/setup-aws-from-admin.sh
```

Verification note:

- By default, the script skips `sts:AssumeRole` verification using your current admin caller.
- This is expected because trust policies are scoped to `droplet-runtime`, not your Identity Center admin role.
- If you intentionally want caller-based assume-role verification, run with:

```bash
VERIFY_ASSUME_ROLE_WITH_CALLER=true ./scripts/setup-aws-from-admin.sh
```

### What the script does

1. Creates or reuses IAM user `droplet-runtime`.
2. Applies least-privilege bootstrap policy (`sts:AssumeRole` + `sts:GetCallerIdentity`) to runtime user.
3. Creates or reuses project S3 bucket and applies baseline controls.
4. Creates or updates app assumed role and inline policy.
5. Creates or updates integration assumed role (with external ID) and inline policy.
6. Runs verification checks for IAM/S3/STS assume-role flows.
7. Prints next-step exports for integration testing.

### How to check success

1. Script exits with code `0`.
2. Log file contains `AWS setup completed successfully`.
3. Manual sanity checks:

```bash
AWS_PROFILE=s3-service-admin aws sts get-caller-identity
AWS_PROFILE=s3-service-admin aws iam get-user --user-name droplet-runtime
AWS_PROFILE=s3-service-admin aws iam get-role --role-name s3-service-bucket-access
AWS_PROFILE=s3-service-admin aws iam get-role --role-name s3-service-it-role
```

## Scope Assumptions

Current backend AWS behavior:

1. Backend loads bootstrap credentials from default AWS SDK chain.
2. Backend assumes bucket role ARNs stored in DB bucket connections.
3. S3 actions run using assumed role credentials.

Current code paths require these AWS APIs:

- STS: `sts:AssumeRole`
- S3 object operations: `s3:PutObject`, `s3:GetObject`, `s3:DeleteObject`
- S3 bucket baseline checks: `s3:GetPublicAccessBlock`, `s3:GetBucketPolicyStatus`, `s3:GetBucketOwnershipControls`

Planned roadmap additions in `s3-service.md` may require:

- `s3:ListBucket` (image listing implementation path)
- FinOps reads: Cost Explorer / Budgets APIs
- Optional CloudWatch/SNS for alerting

## Step 0: Secure Root First

Do these before creating project identities:

1. Enable MFA on root account.
2. Remove root access keys if any exist.
3. Use root only for bootstrap or break-glass.

## Step 1: Create Human Admin Access (Identity Center)

Use IAM Identity Center for human interactive access.

1. Open IAM Identity Center and enable it in `ap-southeast-2`.
2. Create a user (your normal admin identity).
3. Create permission set `s3-service-admin` using either:
- Temporary: `AdministratorAccess` (quick bootstrap)
- Preferred: custom policy from this guide
4. Assign user + permission set to your AWS account.
5. Log in locally:

```bash
aws configure sso --profile s3-service-admin
aws sso login --profile s3-service-admin
export AWS_PROFILE=s3-service-admin
aws sts get-caller-identity
```

> Use this account + script

## Step 2: Define Naming Variables

```bash
export AWS_PROFILE=s3-service-admin
export AWS_REGION=ap-southeast-2
export ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

export PROJECT_PREFIX=s3-service
export BUCKET_NAME=${PROJECT_PREFIX}-data-${ACCOUNT_ID}

# Runtime identities/roles
export DROPLET_USER_NAME=droplet-runtime
export APP_ASSUME_ROLE_NAME=${PROJECT_PREFIX}-bucket-access
export APP_ASSUME_ROLE_ARN=arn:aws:iam::${ACCOUNT_ID}:role/${APP_ASSUME_ROLE_NAME}

# Integration role
export IT_ROLE_NAME=${PROJECT_PREFIX}-it-role
export IT_ROLE_ARN=arn:aws:iam::${ACCOUNT_ID}:role/${IT_ROLE_NAME}
export IT_EXTERNAL_ID=${PROJECT_PREFIX}-it-external
```

## Step 3: Create Runtime IAM User For Droplet

Use this only for non-interactive runtime bootstrap credentials.

```bash
aws iam create-user --user-name "${DROPLET_USER_NAME}"
```

Attach minimal bootstrap policy (assume-role only; JSON below).

```bash
cat > droplet-runtime-bootstrap-policy.json <<EOF
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

aws iam put-user-policy \
  --user-name "${DROPLET_USER_NAME}" \
  --policy-name ${PROJECT_PREFIX}-droplet-bootstrap \
  --policy-document file://droplet-runtime-bootstrap-policy.json
```

Create access key for this user (store securely):

```bash
aws iam create-access-key --user-name "${DROPLET_USER_NAME}"
```

## Step 4: Create Project Bucket And Baseline Controls

```bash
aws s3api create-bucket \
  --bucket "${BUCKET_NAME}" \
  --region "${AWS_REGION}" \
  --create-bucket-configuration LocationConstraint="${AWS_REGION}"

aws s3api put-public-access-block \
  --bucket "${BUCKET_NAME}" \
  --public-access-block-configuration BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true

aws s3api put-bucket-ownership-controls \
  --bucket "${BUCKET_NAME}" \
  --ownership-controls 'Rules=[{ObjectOwnership=BucketOwnerEnforced}]'
```

## Step 5: Create App Assumed Role (Used By Backend)

### 5.1 Trust policy

Trust the droplet runtime IAM user to assume this role.

```bash
cat > app-assume-role-trust.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::${ACCOUNT_ID}:user/${DROPLET_USER_NAME}"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
EOF

aws iam create-role \
  --role-name "${APP_ASSUME_ROLE_NAME}" \
  --assume-role-policy-document file://app-assume-role-trust.json
```

### 5.2 Least-privilege permissions policy for backend role

This policy matches current implementation and near-term roadmap.

```bash
cat > app-assume-role-policy.json <<EOF
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

aws iam put-role-policy \
  --role-name "${APP_ASSUME_ROLE_NAME}" \
  --policy-name ${PROJECT_PREFIX}-bucket-access \
  --policy-document file://app-assume-role-policy.json
```

## Step 6: Create Integration-Test Assumed Role

### 6.1 Trust policy with external ID

```bash
cat > it-role-trust.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::${ACCOUNT_ID}:user/${DROPLET_USER_NAME}"
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

aws iam create-role \
  --role-name "${IT_ROLE_NAME}" \
  --assume-role-policy-document file://it-role-trust.json
```

### 6.2 Integration role policy

Scoped to test prefixes only.

```bash
cat > it-role-policy.json <<EOF
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

aws iam put-role-policy \
  --role-name "${IT_ROLE_NAME}" \
  --policy-name ${PROJECT_PREFIX}-it-access \
  --policy-document file://it-role-policy.json
```

## Step 7: Configure Droplet Runtime Profile

On droplet:

```bash
aws configure --profile droplet-runtime
```

Use the access key created for `${DROPLET_USER_NAME}`.

Verify:

```bash
AWS_PROFILE=droplet-runtime aws sts get-caller-identity
```

Verify assume-role path:

```bash
AWS_PROFILE=droplet-runtime aws sts assume-role \
  --role-arn "${APP_ASSUME_ROLE_ARN}" \
  --role-session-name app-runtime-check
```

Verify integration role path:

```bash
AWS_PROFILE=droplet-runtime aws sts assume-role \
  --role-arn "${IT_ROLE_ARN}" \
  --role-session-name it-runtime-check \
  --external-id "${IT_EXTERNAL_ID}"
```

## Step 8: Run Project Integration Test

```bash
AWS_PROFILE=droplet-runtime \
RUN_AWS_INTEGRATION=true \
AWS_IT_BUCKET="${BUCKET_NAME}" \
AWS_IT_REGION="${AWS_REGION}" \
AWS_IT_ROLE_ARN="${IT_ROLE_ARN}" \
AWS_IT_EXTERNAL_ID="${IT_EXTERNAL_ID}" \
./scripts/run-aws-integration.sh
```

## Step 9: Admin Permission Set (Least Privilege) For Ongoing Human Setup

After bootstrap, replace broad admin access for `s3-service-admin` with a custom permission set policy such as:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "ManageProjectRoles",
      "Effect": "Allow",
      "Action": [
        "iam:CreateRole",
        "iam:DeleteRole",
        "iam:GetRole",
        "iam:UpdateAssumeRolePolicy",
        "iam:PutRolePolicy",
        "iam:DeleteRolePolicy",
        "iam:ListRolePolicies"
      ],
      "Resource": "arn:aws:iam::*:role/s3-service-*"
    },
    {
      "Sid": "ManageProjectRuntimeUserPolicyOnly",
      "Effect": "Allow",
      "Action": [
        "iam:GetUser",
        "iam:PutUserPolicy",
        "iam:DeleteUserPolicy"
      ],
      "Resource": "arn:aws:iam::*:user/droplet-runtime"
    },
    {
      "Sid": "BucketBaselineAndPolicyReads",
      "Effect": "Allow",
      "Action": [
        "s3:CreateBucket",
        "s3:DeleteBucket",
        "s3:PutPublicAccessBlock",
        "s3:GetPublicAccessBlock",
        "s3:PutBucketOwnershipControls",
        "s3:GetBucketOwnershipControls",
        "s3:GetBucketPolicyStatus"
      ],
      "Resource": [
        "arn:aws:s3:::s3-service-*"
      ]
    },
    {
      "Sid": "AllowStsChecks",
      "Effect": "Allow",
      "Action": [
        "sts:GetCallerIdentity",
        "sts:AssumeRole"
      ],
      "Resource": "*"
    }
  ]
}
```

Add optional FinOps scope if/when Epic 5 jobs are implemented:

- `ce:GetCostAndUsage`
- `ce:GetCostForecast`
- `ce:GetDimensionValues`
- `budgets:ViewBudget`
- `budgets:DescribeBudget`

Add optional observability scope if/when Epic 6 alerting is implemented:

- `cloudwatch:PutMetricAlarm`
- `cloudwatch:DeleteAlarms`
- `cloudwatch:DescribeAlarms`
- `cloudwatch:GetMetricData`
- `cloudwatch:ListMetrics`
- `sns:CreateTopic`, `sns:Subscribe`, `sns:Publish` (only if SNS route is used)

## Operational Notes

- Keep runtime credentials out of source control.
- Restrict role trust principals to exact ARNs.
- Use external ID on integration or cross-account roles.
- Keep app and integration roles separate.
- Prefer narrower bucket/prefix scopes over wildcard resources.
