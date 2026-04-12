package router_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"

	"s3-service/internal/adapters"
	"s3-service/internal/auth"
	"s3-service/internal/database"
	httpmiddleware "s3-service/internal/httpapi/middleware"
	"s3-service/internal/httpapi/router"
	"s3-service/internal/s3"
	"s3-service/internal/service"
)

type awsRouterIntegrationConfig struct {
	bucketName string
	region     string
	roleARN    string
	externalID *string
}

func loadAWSRouterIntegrationConfig(t *testing.T) awsRouterIntegrationConfig {
	t.Helper()

	if strings.ToLower(strings.TrimSpace(os.Getenv("RUN_AWS_INTEGRATION"))) != "true" {
		t.Skip("set RUN_AWS_INTEGRATION=true to run real AWS router integration test")
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

	return awsRouterIntegrationConfig{
		bucketName: bucketName,
		region:     region,
		roleARN:    roleARN,
		externalID: externalID,
	}
}

type awsRouterIntegrationRepo struct {
	bucket database.BucketConnection
	policy database.EffectiveAuthorizationPolicy
}

func (r *awsRouterIntegrationRepo) ListActiveBucketsForConnectionScope(_ context.Context, _, _ string) ([]database.BucketConnection, error) {
	return []database.BucketConnection{r.bucket}, nil
}

func (r *awsRouterIntegrationRepo) GetEffectiveAuthorizationPolicy(_ context.Context, lookup database.AuthorizationPolicyLookup) (database.EffectiveAuthorizationPolicy, error) {
	if lookup.BucketName != r.bucket.BucketName {
		return database.EffectiveAuthorizationPolicy{}, database.ErrPolicyNotFound
	}
	return r.policy, nil
}

func awsClaimsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		claims := auth.Claims{
			Subject:       "user-1",
			AppID:         "app-1",
			ProjectID:     "project-1",
			Role:          auth.RoleProjectClient,
			PrincipalType: auth.PrincipalTypeUser,
		}
		next.ServeHTTP(w, req.WithContext(httpmiddleware.ContextWithClaims(req.Context(), claims)))
	})
}

func TestAWSIntegration_RouterImageFlow(t *testing.T) {
	cfg := loadAWSRouterIntegrationConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	keyPrefix := "it/router/" + time.Now().UTC().Format("20060102-150405") + "-" + strings.ReplaceAll(time.Now().UTC().Format("150405.000000000"), ".", "")
	objectKey := keyPrefix + "/image.jpg"
	imageID := base64.RawURLEncoding.EncodeToString([]byte(cfg.bucketName + ":" + objectKey))
	payload := "aws-router-integration-payload"

	repo := &awsRouterIntegrationRepo{
		bucket: database.BucketConnection{
			BucketName:      cfg.bucketName,
			Region:          cfg.region,
			RoleARN:         cfg.roleARN,
			ExternalID:      cfg.externalID,
			AllowedPrefixes: []string{keyPrefix + "/"},
		},
		policy: database.EffectiveAuthorizationPolicy{
			CanRead:            true,
			CanWrite:           true,
			CanDelete:          true,
			CanList:            true,
			ConnectionPrefixes: []string{keyPrefix + "/"},
			PrincipalPrefixes:  []string{keyPrefix + "/"},
		},
	}

	sessionCache, err := s3.NewAssumeRoleSessionCache(ctx, aws.Config{})
	if err != nil {
		t.Fatalf("create assume role session cache: %v", err)
	}

	uploadSvc := service.NewObjectUploadService(repo, adapters.NewS3ObjectUploaderAdapter(s3.NewUploadHelper(sessionCache)))
	deleteSvc := service.NewObjectDeleteService(repo, adapters.NewS3ObjectDeleterAdapter(s3.NewDeleteHelper(sessionCache)))
	presignSvc := service.NewObjectPresignService(repo, adapters.NewS3ObjectPresignerAdapter(s3.NewPresignHelper(sessionCache)))
	listSvc := service.NewObjectListService(repo, repo, adapters.NewS3ObjectListerAdapter(s3.NewListHelper(sessionCache)))
	readSvc := service.NewObjectReadService(repo, adapters.NewS3ObjectReaderAdapter(s3.NewGetHelper(sessionCache)))
	authzSvc := service.NewAuthorizationService(repo)

	r := router.NewRouter(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		awsClaimsMiddleware,
		nil,
		authzSvc,
		uploadSvc,
		deleteSvc,
		presignSvc,
		listSvc,
		readSvc,
		nil,
	)

	uploadReq := httptest.NewRequest(http.MethodPost, "/v1/objects/upload", strings.NewReader(`{"bucket_name":"`+cfg.bucketName+`","object_key":"`+objectKey+`","content_type":"image/jpeg","content_b64":"`+base64.StdEncoding.EncodeToString([]byte(payload))+`"}`))
	uploadReq.RemoteAddr = "203.0.113.10:43210"
	uploadReq.Header.Set("Content-Type", "application/json")
	uploadRec := httptest.NewRecorder()
	r.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("upload failed: status=%d body=%s", uploadRec.Code, uploadRec.Body.String())
	}

	presignGetReq := httptest.NewRequest(http.MethodPost, "/v1/objects/presign-download", strings.NewReader(`{"bucket_name":"`+cfg.bucketName+`","object_key":"`+objectKey+`","expires_in_seconds":120}`))
	presignGetReq.RemoteAddr = "203.0.113.10:43210"
	presignGetReq.Header.Set("Content-Type", "application/json")
	presignGetRec := httptest.NewRecorder()
	r.ServeHTTP(presignGetRec, presignGetReq)
	if presignGetRec.Code != http.StatusOK {
		t.Fatalf("presign download failed: status=%d body=%s", presignGetRec.Code, presignGetRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/images", nil)
	listReq.RemoteAddr = "203.0.113.10:43210"
	listRec := httptest.NewRecorder()
	r.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list images failed: status=%d body=%s", listRec.Code, listRec.Body.String())
	}
	var listEnvelope struct {
		Data *struct {
			Images []struct {
				ID string `json:"id"`
			} `json:"images"`
		} `json:"data"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&listEnvelope); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if listEnvelope.Data == nil || len(listEnvelope.Data.Images) == 0 {
		t.Fatalf("expected discovery images, got %+v", listEnvelope)
	}

	resolveReq := httptest.NewRequest(http.MethodGet, "/v1/images?ids="+imageID, nil)
	resolveReq.RemoteAddr = "203.0.113.10:43210"
	resolveRec := httptest.NewRecorder()
	r.ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("resolve images failed: status=%d body=%s", resolveRec.Code, resolveRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/images/"+imageID, nil)
	getReq.RemoteAddr = "203.0.113.10:43210"
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get image failed: status=%d body=%s", getRec.Code, getRec.Body.String())
	}
	if getRec.Body.String() != payload {
		t.Fatalf("unexpected image payload: got=%q want=%q", getRec.Body.String(), payload)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/images/"+imageID, nil)
	deleteReq.RemoteAddr = "203.0.113.10:43210"
	deleteRec := httptest.NewRecorder()
	r.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete image failed: status=%d body=%s", deleteRec.Code, deleteRec.Body.String())
	}

	getAfterDeleteReq := httptest.NewRequest(http.MethodGet, "/v1/images/"+imageID, nil)
	getAfterDeleteReq.RemoteAddr = "203.0.113.10:43210"
	getAfterDeleteRec := httptest.NewRecorder()
	r.ServeHTTP(getAfterDeleteRec, getAfterDeleteReq)
	if getAfterDeleteRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got status=%d body=%s", getAfterDeleteRec.Code, getAfterDeleteRec.Body.String())
	}
}
