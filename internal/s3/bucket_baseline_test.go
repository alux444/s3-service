package s3

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type stubRoleConfigProvider struct {
	cfg aws.Config
	err error
}

func (s *stubRoleConfigProvider) ConfigForRole(context.Context, BucketRoleReference) (aws.Config, error) {
	if s.err != nil {
		return aws.Config{}, s.err
	}
	return s.cfg, nil
}

type stubBucketSecurityClient struct {
	publicAccessBlockOut *awss3.GetPublicAccessBlockOutput
	publicAccessBlockErr error

	policyStatusOut *awss3.GetBucketPolicyStatusOutput
	policyStatusErr error

	ownershipControlsOut *awss3.GetBucketOwnershipControlsOutput
	ownershipControlsErr error
}

func (s *stubBucketSecurityClient) GetPublicAccessBlock(context.Context, *awss3.GetPublicAccessBlockInput, ...func(*awss3.Options)) (*awss3.GetPublicAccessBlockOutput, error) {
	if s.publicAccessBlockErr != nil {
		return nil, s.publicAccessBlockErr
	}
	return s.publicAccessBlockOut, nil
}

func (s *stubBucketSecurityClient) GetBucketPolicyStatus(context.Context, *awss3.GetBucketPolicyStatusInput, ...func(*awss3.Options)) (*awss3.GetBucketPolicyStatusOutput, error) {
	if s.policyStatusErr != nil {
		return nil, s.policyStatusErr
	}
	return s.policyStatusOut, nil
}

func (s *stubBucketSecurityClient) GetBucketOwnershipControls(context.Context, *awss3.GetBucketOwnershipControlsInput, ...func(*awss3.Options)) (*awss3.GetBucketOwnershipControlsOutput, error) {
	if s.ownershipControlsErr != nil {
		return nil, s.ownershipControlsErr
	}
	return s.ownershipControlsOut, nil
}

type testAPIError struct {
	code string
	msg  string
}

func (e testAPIError) ErrorCode() string             { return e.code }
func (e testAPIError) ErrorMessage() string          { return e.msg }
func (e testAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }
func (e testAPIError) Error() string                 { return e.code + ": " + e.msg }

func TestBucketSecurityBaselineChecker_ValidateBucketConnection(t *testing.T) {
	t.Run("accepts secure private bucket", func(t *testing.T) {
		provider := &stubRoleConfigProvider{cfg: aws.Config{Region: "us-east-1"}}
		client := &stubBucketSecurityClient{
			publicAccessBlockOut: &awss3.GetPublicAccessBlockOutput{PublicAccessBlockConfiguration: &types.PublicAccessBlockConfiguration{
				BlockPublicAcls:       aws.Bool(true),
				IgnorePublicAcls:      aws.Bool(true),
				BlockPublicPolicy:     aws.Bool(true),
				RestrictPublicBuckets: aws.Bool(true),
			}},
			policyStatusOut:      &awss3.GetBucketPolicyStatusOutput{PolicyStatus: &types.PolicyStatus{IsPublic: aws.Bool(false)}},
			ownershipControlsOut: &awss3.GetBucketOwnershipControlsOutput{OwnershipControls: &types.OwnershipControls{Rules: []types.OwnershipControlsRule{{ObjectOwnership: types.ObjectOwnershipBucketOwnerEnforced}}}},
		}

		checker := NewBucketSecurityBaselineChecker(provider, WithBucketSecurityClientFactory(func(aws.Config) bucketSecurityClient {
			return client
		}))

		err := checker.ValidateBucketConnection(context.Background(), "bucket-a", "us-east-1", "arn:aws:iam::123456789012:role/s3", nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("returns baseline violation for insecure controls", func(t *testing.T) {
		provider := &stubRoleConfigProvider{cfg: aws.Config{Region: "us-east-1"}}
		client := &stubBucketSecurityClient{
			publicAccessBlockOut: &awss3.GetPublicAccessBlockOutput{PublicAccessBlockConfiguration: &types.PublicAccessBlockConfiguration{
				BlockPublicAcls:       aws.Bool(true),
				IgnorePublicAcls:      aws.Bool(false),
				BlockPublicPolicy:     aws.Bool(true),
				RestrictPublicBuckets: aws.Bool(true),
			}},
			policyStatusOut:      &awss3.GetBucketPolicyStatusOutput{PolicyStatus: &types.PolicyStatus{IsPublic: aws.Bool(true)}},
			ownershipControlsOut: &awss3.GetBucketOwnershipControlsOutput{OwnershipControls: &types.OwnershipControls{Rules: []types.OwnershipControlsRule{{ObjectOwnership: types.ObjectOwnershipObjectWriter}}}},
		}

		checker := NewBucketSecurityBaselineChecker(provider, WithBucketSecurityClientFactory(func(aws.Config) bucketSecurityClient {
			return client
		}))

		err := checker.ValidateBucketConnection(context.Background(), "bucket-a", "us-east-1", "arn:aws:iam::123456789012:role/s3", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrBucketSecurityBaselineViolation) {
			t.Fatalf("expected ErrBucketSecurityBaselineViolation, got %v", err)
		}
		if !strings.Contains(err.Error(), "public") || !strings.Contains(err.Error(), "BucketOwnerEnforced") {
			t.Fatalf("expected detailed baseline reasons, got %v", err)
		}
	})

	t.Run("returns wrapped error when role config fails", func(t *testing.T) {
		provider := &stubRoleConfigProvider{err: errors.New("assume role failed")}
		checker := NewBucketSecurityBaselineChecker(provider)

		err := checker.ValidateBucketConnection(context.Background(), "bucket-a", "us-east-1", "arn:aws:iam::123456789012:role/s3", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "build role config") {
			t.Fatalf("expected wrapped role config error, got %v", err)
		}
	})

	t.Run("treats missing ownership controls as baseline violation", func(t *testing.T) {
		provider := &stubRoleConfigProvider{cfg: aws.Config{Region: "us-east-1"}}
		client := &stubBucketSecurityClient{
			publicAccessBlockOut: &awss3.GetPublicAccessBlockOutput{PublicAccessBlockConfiguration: &types.PublicAccessBlockConfiguration{
				BlockPublicAcls:       aws.Bool(true),
				IgnorePublicAcls:      aws.Bool(true),
				BlockPublicPolicy:     aws.Bool(true),
				RestrictPublicBuckets: aws.Bool(true),
			}},
			policyStatusErr:      testAPIError{code: "NoSuchBucketPolicy", msg: "no policy"},
			ownershipControlsErr: testAPIError{code: "OwnershipControlsNotFoundError", msg: "missing"},
		}

		checker := NewBucketSecurityBaselineChecker(provider, WithBucketSecurityClientFactory(func(aws.Config) bucketSecurityClient {
			return client
		}))

		err := checker.ValidateBucketConnection(context.Background(), "bucket-a", "us-east-1", "arn:aws:iam::123456789012:role/s3", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrBucketSecurityBaselineViolation) {
			t.Fatalf("expected baseline violation, got %v", err)
		}
		if !strings.Contains(err.Error(), "ownership") {
			t.Fatalf("expected ownership reason, got %v", err)
		}
	})
}
