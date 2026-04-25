package saladclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is the HTTP client for s3-service API
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a new s3-service client
func NewClient(baseURL string, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientWithHTTPClient creates a new client with a custom HTTP client
func NewClientWithHTTPClient(baseURL string, token string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    baseURL,
		token:      token,
		httpClient: httpClient,
	}
}

// doRequest performs an HTTP request with bearer token authentication
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, responseType interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		bodyData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyData)
	}

	url := fmt.Sprintf("%s%s", c.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr APIResponse
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Error != nil {
			return fmt.Errorf("API error (HTTP %d): %s - %s", resp.StatusCode, apiErr.Error.Code, apiErr.Error.Message)
		}
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(respBody))
	}

	if responseType != nil {
		if err := json.Unmarshal(respBody, responseType); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// Health checks the health of the service
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	var resp HealthResponse
	path := "/health"
	url := fmt.Sprintf("%s%s", c.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// AuthCheck verifies the current authentication token and returns claims
func (c *Client) AuthCheck(ctx context.Context) (*AuthCheckResponse, error) {
	var resp AuthCheckResponse
	if err := c.doRequest(ctx, http.MethodGet, "/v1/auth-check", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateBucketConnection creates a new bucket connection in the service
func (c *Client) CreateBucketConnection(ctx context.Context, req *CreateBucketConnectionRequest) (*CreateBucketConnectionResponse, error) {
	var resp CreateBucketConnectionResponse
	if err := c.doRequest(ctx, http.MethodPost, "/v1/bucket-connections", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListBucketConnections lists all bucket connections for the authenticated scope
func (c *Client) ListBucketConnections(ctx context.Context) (*ListBucketConnectionsResponse, error) {
	var resp ListBucketConnectionsResponse
	if err := c.doRequest(ctx, http.MethodGet, "/v1/bucket-connections", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpsertAccessPolicy creates or updates an access policy
func (c *Client) UpsertAccessPolicy(ctx context.Context, req *UpsertAccessPolicyRequest) (*UpsertAccessPolicyResponse, error) {
	var resp UpsertAccessPolicyResponse
	if err := c.doRequest(ctx, http.MethodPost, "/v1/access-policies", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UploadObject uploads an object to a bucket
func (c *Client) UploadObject(ctx context.Context, req *ObjectUploadRequest) (*ObjectUploadResponse, error) {
	var resp ObjectUploadResponse
	if err := c.doRequest(ctx, http.MethodPost, "/v1/objects/upload", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UploadObjectWithData uploads an object with raw data (base64 encoded internally)
func (c *Client) UploadObjectWithData(ctx context.Context, bucketName, objectKey, contentType string, data []byte, metadata map[string]string) (*ObjectUploadResponse, error) {
	req := &ObjectUploadRequest{
		BucketName:  bucketName,
		ObjectKey:   objectKey,
		ContentType: contentType,
		ContentB64:  base64.StdEncoding.EncodeToString(data),
		Metadata:    metadata,
	}
	return c.UploadObject(ctx, req)
}

// DeleteObject deletes an object from a bucket
func (c *Client) DeleteObject(ctx context.Context, bucketName, objectKey string) (*ObjectDeleteResponse, error) {
	var resp ObjectDeleteResponse
	params := url.Values{}
	params.Add("bucket_name", bucketName)
	params.Add("object_key", objectKey)
	path := fmt.Sprintf("/v1/objects?%s", params.Encode())
	if err := c.doRequest(ctx, http.MethodDelete, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PresignUploadURL generates a presigned URL for uploading an object
func (c *Client) PresignUploadURL(ctx context.Context, bucketName, objectKey string) (*PresignResponse, error) {
	req := &PresignUploadRequest{
		BucketName: bucketName,
		ObjectKey:  objectKey,
	}
	var resp PresignResponse
	if err := c.doRequest(ctx, http.MethodPost, "/v1/objects/presign-upload", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PresignDownloadURL generates a presigned URL for downloading an object
func (c *Client) PresignDownloadURL(ctx context.Context, bucketName, objectKey string) (*PresignResponse, error) {
	req := &PresignDownloadRequest{
		BucketName: bucketName,
		ObjectKey:  objectKey,
	}
	var resp PresignResponse
	if err := c.doRequest(ctx, http.MethodPost, "/v1/objects/presign-download", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListImages lists all available images for the authenticated scope
func (c *Client) ListImages(ctx context.Context) (*ListImagesResponse, error) {
	var resp ListImagesResponse
	if err := c.doRequest(ctx, http.MethodGet, "/v1/images", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListImagesByIDs retrieves specific images by their IDs
func (c *Client) ListImagesByIDs(ctx context.Context, ids []string) (*ListImagesResponse, error) {
	var resp ListImagesResponse
	params := url.Values{}
	for _, id := range ids {
		params.Add("ids", id)
	}
	path := fmt.Sprintf("/v1/images?%s", params.Encode())
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetImage retrieves a specific image by ID
func (c *Client) GetImage(ctx context.Context, imageID string) ([]byte, error) {
	path := fmt.Sprintf("/v1/images/%s", url.PathEscape(imageID))
	url := fmt.Sprintf("%s%s", c.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// DeleteImage deletes an image by ID
func (c *Client) DeleteImage(ctx context.Context, imageID string) (*ObjectDeleteResponse, error) {
	var resp ObjectDeleteResponse
	path := fmt.Sprintf("/v1/images/%s", url.PathEscape(imageID))
	if err := c.doRequest(ctx, http.MethodDelete, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
