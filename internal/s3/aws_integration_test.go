package s3

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

type awsIntegrationConfig struct {
	bucketName string
	region     string
	roleARN    string
	externalID *string
}

func loadAWSIntegrationConfig(t *testing.T) awsIntegrationConfig {
	t.Helper()

	if strings.ToLower(strings.TrimSpace(os.Getenv("RUN_AWS_INTEGRATION"))) != "true" {
		t.Skip("set RUN_AWS_INTEGRATION=true to run real AWS integration tests")
	}

	bucketName := strings.TrimSpace(os.Getenv("AWS_IT_BUCKET"))
	region := strings.TrimSpace(os.Getenv("AWS_IT_REGION"))
	roleARN := strings.TrimSpace(os.Getenv("AWS_IT_ROLE_ARN"))
	externalIDRaw := strings.TrimSpace(os.Getenv("AWS_IT_EXTERNAL_ID"))

	missing := make([]string, 0, 3)
	if bucketName == "" {
		missing = append(missing, "AWS_IT_BUCKET")
	}
	if region == "" {
		missing = append(missing, "AWS_IT_REGION")
	}
	if roleARN == "" {
		missing = append(missing, "AWS_IT_ROLE_ARN")
	}
	if len(missing) > 0 {
		t.Skipf("missing required env var(s): %s", strings.Join(missing, ", "))
	}

	var externalID *string
	if externalIDRaw != "" {
		externalID = &externalIDRaw
	}

	return awsIntegrationConfig{
		bucketName: bucketName,
		region:     region,
		roleARN:    roleARN,
		externalID: externalID,
	}
}

func TestAWSIntegration_BucketBaselineUploadPresignDelete(t *testing.T) {
	cfg := loadAWSIntegrationConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	sessionCache, err := NewAssumeRoleSessionCache(context.Background(), aws.Config{}, WithAssumeRoleRetryPolicy(retryPolicy{maxAttempts: 3, initialDelay: 100 * time.Millisecond, maxDelay: 500 * time.Millisecond}))
	if err != nil {
		t.Fatalf("create assume role session cache: %v", err)
	}

	baselineChecker := NewBucketSecurityBaselineChecker(sessionCache)
	uploadHelper := NewUploadHelper(sessionCache)
	deleteHelper := NewDeleteHelper(sessionCache)
	presignHelper := NewPresignHelper(sessionCache)

	if err := baselineChecker.ValidateBucketConnection(ctx, cfg.bucketName, cfg.region, cfg.roleARN, cfg.externalID); err != nil {
		t.Fatalf("bucket baseline check failed: %v", err)
	}

	keyPrefix := fmt.Sprintf("it/runs/%d", time.Now().UnixNano())
	uploadKey := keyPrefix + "/upload-object.png"

	payload := []byte("aws integration payload")
	uploadResult, err := uploadHelper.UploadObject(ctx, UploadObjectInput{
		BucketName:  cfg.bucketName,
		ObjectKey:   uploadKey,
		Region:      cfg.region,
		RoleARN:     cfg.roleARN,
		ExternalID:  cfg.externalID,
		ContentType: "image/png",
		Body:        payload,
		Metadata:    map[string]string{"integration": "true"},
	})
	if err != nil {
		t.Fatalf("upload object failed: %v", err)
	}
	if uploadResult.Size != int64(len(payload)) {
		t.Fatalf("unexpected upload size: got=%d want=%d", uploadResult.Size, len(payload))
	}

	putPresignResult, err := presignHelper.PresignObject(ctx, PresignObjectInput{
		BucketName:  cfg.bucketName,
		ObjectKey:   keyPrefix + "/presign-put.png",
		Region:      cfg.region,
		RoleARN:     cfg.roleARN,
		ExternalID:  cfg.externalID,
		Method:      "PUT",
		ExpiresIn:   60 * time.Second,
		ContentType: "image/png",
	})
	if err != nil {
		t.Fatalf("presign PUT failed: %v", err)
	}
	assertSignedURL(t, putPresignResult.URL)

	getPresignResult, err := presignHelper.PresignObject(ctx, PresignObjectInput{
		BucketName: cfg.bucketName,
		ObjectKey:  uploadKey,
		Region:     cfg.region,
		RoleARN:    cfg.roleARN,
		ExternalID: cfg.externalID,
		Method:     "GET",
		ExpiresIn:  60 * time.Second,
	})
	if err != nil {
		t.Fatalf("presign GET failed: %v", err)
	}
	assertSignedURL(t, getPresignResult.URL)

	deleteResult, err := deleteHelper.DeleteObject(ctx, DeleteObjectInput{
		BucketName:      cfg.bucketName,
		ObjectKey:       uploadKey,
		Region:          cfg.region,
		RoleARN:         cfg.roleARN,
		ExternalID:      cfg.externalID,
		AllowedPrefixes: []string{keyPrefix + "/"},
	})
	if err != nil {
		t.Fatalf("delete object failed: %v", err)
	}
	if !deleteResult.Deleted {
		t.Fatal("expected Deleted=true")
	}

	// Second delete verifies idempotent soft-fail behavior for missing keys.
	secondDeleteResult, err := deleteHelper.DeleteObject(ctx, DeleteObjectInput{
		BucketName:      cfg.bucketName,
		ObjectKey:       uploadKey,
		Region:          cfg.region,
		RoleARN:         cfg.roleARN,
		ExternalID:      cfg.externalID,
		AllowedPrefixes: []string{keyPrefix + "/"},
	})
	if err != nil {
		t.Fatalf("second delete object failed: %v", err)
	}
	if !secondDeleteResult.Deleted {
		t.Fatal("expected Deleted=true on second delete")
	}
}

func assertSignedURL(t *testing.T, raw string) {
	t.Helper()
	if strings.TrimSpace(raw) == "" {
		t.Fatal("expected non-empty presigned URL")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("invalid presigned URL: %v", err)
	}
	if parsed.Scheme != "https" {
		t.Fatalf("expected https presigned URL, got %q", parsed.Scheme)
	}
	if parsed.Query().Get("X-Amz-Signature") == "" {
		t.Fatal("expected X-Amz-Signature in presigned URL")
	}
}
