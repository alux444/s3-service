package s3

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

var ErrBucketSecurityBaselineViolation = errors.New("bucket security baseline violation")

type BucketSecurityBaselineError struct {
	BucketName string
	Reasons    []string
}

func (e *BucketSecurityBaselineError) Error() string {
	if len(e.Reasons) == 0 {
		return "bucket does not meet required security baseline"
	}
	return fmt.Sprintf("bucket %q does not meet required security baseline: %s", e.BucketName, strings.Join(e.Reasons, "; "))
}

func (e *BucketSecurityBaselineError) Unwrap() error {
	return ErrBucketSecurityBaselineViolation
}

type roleConfigProvider interface {
	ConfigForRole(ctx context.Context, ref BucketRoleReference) (aws.Config, error)
}

type bucketSecurityClient interface {
	GetPublicAccessBlock(ctx context.Context, params *awss3.GetPublicAccessBlockInput, optFns ...func(*awss3.Options)) (*awss3.GetPublicAccessBlockOutput, error)
	GetBucketPolicyStatus(ctx context.Context, params *awss3.GetBucketPolicyStatusInput, optFns ...func(*awss3.Options)) (*awss3.GetBucketPolicyStatusOutput, error)
	GetBucketOwnershipControls(ctx context.Context, params *awss3.GetBucketOwnershipControlsInput, optFns ...func(*awss3.Options)) (*awss3.GetBucketOwnershipControlsOutput, error)
}

type bucketSecurityClientFactory func(cfg aws.Config) bucketSecurityClient

type BucketSecurityBaselineChecker struct {
	sessions      roleConfigProvider
	clientFactory bucketSecurityClientFactory
	retryPolicy   retryPolicy
}

type BucketSecurityBaselineCheckerOption func(*BucketSecurityBaselineChecker)

func WithBucketBaselineRetryPolicy(policy retryPolicy) BucketSecurityBaselineCheckerOption {
	return func(c *BucketSecurityBaselineChecker) {
		c.retryPolicy = policy
	}
}

func WithBucketSecurityClientFactory(factory bucketSecurityClientFactory) BucketSecurityBaselineCheckerOption {
	return func(c *BucketSecurityBaselineChecker) {
		if factory != nil {
			c.clientFactory = factory
		}
	}
}

func NewBucketSecurityBaselineChecker(sessions roleConfigProvider, opts ...BucketSecurityBaselineCheckerOption) *BucketSecurityBaselineChecker {
	checker := &BucketSecurityBaselineChecker{
		sessions:    sessions,
		retryPolicy: defaultRetryPolicy(),
		clientFactory: func(cfg aws.Config) bucketSecurityClient {
			return awss3.NewFromConfig(cfg)
		},
	}

	for _, opt := range opts {
		opt(checker)
	}

	return checker
}

func (c *BucketSecurityBaselineChecker) ValidateBucketConnection(ctx context.Context, bucketName, region, roleARN string, externalID *string) error {
	if bucketName == "" || region == "" || roleARN == "" {
		return fmt.Errorf("%w: bucket_name, region, and role_arn are required", ErrInvalidAssumeRoleInput)
	}

	cfg, err := c.sessions.ConfigForRole(ctx, BucketRoleReference{Region: region, RoleARN: roleARN, ExternalID: externalID})
	if err != nil {
		return fmt.Errorf("build role config for baseline check: %w", err)
	}

	client := c.clientFactory(cfg)
	var reasons []string

	publicAccessBlock, err := retryAWS(ctx, c.retryPolicy, func() (*awss3.GetPublicAccessBlockOutput, error) {
		return client.GetPublicAccessBlock(ctx, &awss3.GetPublicAccessBlockInput{Bucket: aws.String(bucketName)})
	})
	if err != nil {
		if apiErrorCode(err) == "NoSuchPublicAccessBlockConfiguration" {
			reasons = append(reasons, "bucket public access block configuration is missing")
		} else {
			return fmt.Errorf("check public access block: %w", err)
		}
	} else if !hasStrictPublicAccessBlock(publicAccessBlock.PublicAccessBlockConfiguration) {
		reasons = append(reasons, "bucket must enable all public access block settings")
	}

	policyStatus, err := retryAWS(ctx, c.retryPolicy, func() (*awss3.GetBucketPolicyStatusOutput, error) {
		return client.GetBucketPolicyStatus(ctx, &awss3.GetBucketPolicyStatusInput{Bucket: aws.String(bucketName)})
	})
	if err != nil {
		// No bucket policy is acceptable for a private baseline.
		if apiErrorCode(err) != "NoSuchBucketPolicy" {
			return fmt.Errorf("check bucket policy status: %w", err)
		}
	} else if policyStatus.PolicyStatus != nil && aws.ToBool(policyStatus.PolicyStatus.IsPublic) {
		reasons = append(reasons, "bucket policy allows public access")
	}

	ownership, err := retryAWS(ctx, c.retryPolicy, func() (*awss3.GetBucketOwnershipControlsOutput, error) {
		return client.GetBucketOwnershipControls(ctx, &awss3.GetBucketOwnershipControlsInput{Bucket: aws.String(bucketName)})
	})
	if err != nil {
		if apiErrorCode(err) == "OwnershipControlsNotFoundError" {
			reasons = append(reasons, "bucket ownership controls are not configured")
		} else {
			return fmt.Errorf("check bucket ownership controls: %w", err)
		}
	} else if !isBucketOwnerEnforced(ownership.OwnershipControls) {
		reasons = append(reasons, "bucket object ownership must be BucketOwnerEnforced")
	}

	if len(reasons) > 0 {
		return &BucketSecurityBaselineError{BucketName: bucketName, Reasons: reasons}
	}

	return nil
}

func hasStrictPublicAccessBlock(cfg *types.PublicAccessBlockConfiguration) bool {
	if cfg == nil {
		return false
	}

	return aws.ToBool(cfg.BlockPublicAcls) &&
		aws.ToBool(cfg.IgnorePublicAcls) &&
		aws.ToBool(cfg.BlockPublicPolicy) &&
		aws.ToBool(cfg.RestrictPublicBuckets)
}

func isBucketOwnerEnforced(controls *types.OwnershipControls) bool {
	if controls == nil {
		return false
	}

	for _, rule := range controls.Rules {
		if rule.ObjectOwnership == types.ObjectOwnershipBucketOwnerEnforced {
			return true
		}
	}

	return false
}

func apiErrorCode(err error) string {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode()
	}
	return ""
}
