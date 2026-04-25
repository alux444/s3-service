package saladclient

// Health response
type HealthResponse struct {
	Status string `json:"status"`
}

// Auth types
type AuthCheckResponse struct {
	Subject       string `json:"sub"`
	AppID         string `json:"app_id"`
	ProjectID     string `json:"project_id"`
	Role          string `json:"role"`
	PrincipalType string `json:"principal_type"`
}

// Bucket connection types
type CreateBucketConnectionRequest struct {
	BucketName      string   `json:"bucket_name"`
	Region          string   `json:"region"`
	RoleARN         string   `json:"role_arn"`
	ExternalID      *string  `json:"external_id"`
	AllowedPrefixes []string `json:"allowed_prefixes"`
}

type CreateBucketConnectionResponse struct {
	Created bool `json:"created"`
}

type BucketConnection struct {
	BucketName      string   `json:"bucket_name"`
	Region          string   `json:"region"`
	RoleARN         string   `json:"role_arn"`
	ExternalID      *string  `json:"external_id"`
	AllowedPrefixes []string `json:"allowed_prefixes"`
}

type ListBucketConnectionsResponse struct {
	Buckets []BucketConnection `json:"buckets"`
}

// Access policy types
type UpsertAccessPolicyRequest struct {
	BucketName      string   `json:"bucket_name"`
	PrincipalType   string   `json:"principal_type"`
	PrincipalID     string   `json:"principal_id"`
	Role            string   `json:"role"`
	CanRead         *bool    `json:"can_read"`
	CanWrite        *bool    `json:"can_write"`
	CanDelete       *bool    `json:"can_delete"`
	CanList         *bool    `json:"can_list"`
	PrefixAllowlist []string `json:"prefix_allowlist"`
}

type UpsertAccessPolicyResponse struct {
	Upserted bool `json:"upserted"`
}

// Object types
type ObjectUploadRequest struct {
	BucketName  string            `json:"bucket_name"`
	ObjectKey   string            `json:"object_key"`
	ContentType string            `json:"content_type"`
	ContentB64  string            `json:"content_b64"`
	Metadata    map[string]string `json:"metadata"`
}

type ObjectUploadResponse struct {
	Uploaded  bool   `json:"uploaded"`
	Bucket    string `json:"bucket"`
	ObjectKey string `json:"object_key"`
	ETag      string `json:"etag,omitempty"`
	Size      int64  `json:"size,omitempty"`
}

type ObjectDeleteRequest struct {
	BucketName string `json:"bucket_name"`
	ObjectKey  string `json:"object_key"`
}

type ObjectDeleteResponse struct {
	Deleted   bool   `json:"deleted"`
	Bucket    string `json:"bucket"`
	ObjectKey string `json:"object_key"`
}

// Presign types
type PresignUploadRequest struct {
	BucketName string `json:"bucket_name"`
	ObjectKey  string `json:"object_key"`
}

type PresignDownloadRequest struct {
	BucketName string `json:"bucket_name"`
	ObjectKey  string `json:"object_key"`
}

type PresignResponse struct {
	URL       string `json:"url"`
	Method    string `json:"method"`
	ExpiresAt string `json:"expires_at"`
}

// Image types
type ListImagesResponse struct {
	Images []ImageItem `json:"images"`
}

type ImageItem struct {
	ID           string `json:"id"`
	BucketName   string `json:"bucket_name"`
	ObjectKey    string `json:"object_key"`
	SizeBytes    int64  `json:"size_bytes,omitempty"`
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"last_modified,omitempty"`
	URL          string `json:"url"`
}

// Generic API response wrapper (for error handling)
type APIError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type APIResponse struct {
	Data   interface{} `json:"data"`
	Error  *APIError   `json:"error,omitempty"`
	Status int         `json:"status,omitempty"`
}
