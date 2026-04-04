package s3

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

type staticProvider struct {
	value aws.Credentials
	err   error
}

func (p staticProvider) Retrieve(context.Context) (aws.Credentials, error) {
	if p.err != nil {
		return aws.Credentials{}, p.err
	}
	return p.value, nil
}

func TestAssumeRoleSessionCache_ConfigForRoleCachesByRoleKey(t *testing.T) {
	var createCalls int
	cache, err := NewAssumeRoleSessionCache(
		context.Background(),
		aws.Config{},
		WithProviderFactory(func(ref BucketRoleReference, duration time.Duration) (aws.CredentialsProvider, error) {
			createCalls++
			if ref.RoleARN == "" || ref.Region == "" {
				t.Fatalf("provider factory called with invalid reference: %+v", ref)
			}
			if duration != 10*time.Minute {
				t.Fatalf("expected session duration 10m, got %s", duration)
			}
			return staticProvider{value: aws.Credentials{
				AccessKeyID:     "AKIA_TEST",
				SecretAccessKey: "SECRET",
				SessionToken:    "TOKEN",
				CanExpire:       true,
				Expires:         time.Now().Add(5 * time.Minute),
				Source:          "test",
			}}, nil
		}),
		WithSessionDuration(10*time.Minute),
	)
	if err != nil {
		t.Fatalf("NewAssumeRoleSessionCache() error = %v", err)
	}

	ref := BucketRoleReference{Region: "us-east-1", RoleARN: "arn:aws:iam::123456789012:role/test"}
	cfg1, err := cache.ConfigForRole(context.Background(), ref)
	if err != nil {
		t.Fatalf("first ConfigForRole() error = %v", err)
	}
	cfg2, err := cache.ConfigForRole(context.Background(), ref)
	if err != nil {
		t.Fatalf("second ConfigForRole() error = %v", err)
	}

	if createCalls != 1 {
		t.Fatalf("expected provider created once, got %d", createCalls)
	}
	if cfg1.Region != "us-east-1" || cfg2.Region != "us-east-1" {
		t.Fatalf("expected region us-east-1, got cfg1=%q cfg2=%q", cfg1.Region, cfg2.Region)
	}
	if cfg1.Credentials == nil || cfg2.Credentials == nil {
		t.Fatalf("expected credentials cache in returned config")
	}
}

func TestAssumeRoleSessionCache_ConfigForRoleSeparatesByExternalID(t *testing.T) {
	var createCalls int
	cache, err := NewAssumeRoleSessionCache(
		context.Background(),
		aws.Config{},
		WithProviderFactory(func(ref BucketRoleReference, _ time.Duration) (aws.CredentialsProvider, error) {
			createCalls++
			return staticProvider{value: aws.Credentials{
				AccessKeyID:     "AKIA_TEST",
				SecretAccessKey: "SECRET",
				SessionToken:    "TOKEN",
				Source:          ref.RoleARN,
			}}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewAssumeRoleSessionCache() error = %v", err)
	}

	a := "external-a"
	b := "external-b"
	_, err = cache.ConfigForRole(context.Background(), BucketRoleReference{Region: "us-east-1", RoleARN: "arn:aws:iam::123456789012:role/test", ExternalID: &a})
	if err != nil {
		t.Fatalf("first ConfigForRole() error = %v", err)
	}
	_, err = cache.ConfigForRole(context.Background(), BucketRoleReference{Region: "us-east-1", RoleARN: "arn:aws:iam::123456789012:role/test", ExternalID: &b})
	if err != nil {
		t.Fatalf("second ConfigForRole() error = %v", err)
	}

	if createCalls != 2 {
		t.Fatalf("expected provider created twice for different external IDs, got %d", createCalls)
	}
}

func TestAssumeRoleSessionCache_ConfigForRoleValidatesInput(t *testing.T) {
	cache, err := NewAssumeRoleSessionCache(
		context.Background(),
		aws.Config{},
		WithProviderFactory(func(_ BucketRoleReference, _ time.Duration) (aws.CredentialsProvider, error) {
			return staticProvider{value: aws.Credentials{AccessKeyID: "AKIA_TEST", SecretAccessKey: "SECRET"}}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewAssumeRoleSessionCache() error = %v", err)
	}

	_, err = cache.ConfigForRole(context.Background(), BucketRoleReference{Region: "", RoleARN: ""})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "region and role ARN are required") {
		t.Fatalf("expected validation error message, got %v", err)
	}
}

func TestAssumeRoleSessionCache_ConfigForRoleReturnsProviderError(t *testing.T) {
	expected := errors.New("assume role denied")
	cache, err := NewAssumeRoleSessionCache(
		context.Background(),
		aws.Config{},
		WithProviderFactory(func(_ BucketRoleReference, _ time.Duration) (aws.CredentialsProvider, error) {
			return staticProvider{err: expected}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewAssumeRoleSessionCache() error = %v", err)
	}

	_, err = cache.ConfigForRole(context.Background(), BucketRoleReference{Region: "us-east-1", RoleARN: "arn:aws:iam::123456789012:role/test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "assume role denied") {
		t.Fatalf("expected wrapped provider error, got %v", err)
	}
}
