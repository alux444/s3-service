# S3 Service Integration Package

A reusable Go SDK for interacting with the s3-service API from other projects.

## Installation

```bash
go get github.com/yourusername/s3-service/saladclient
```

## Usage

### Basic Setup

```go
package main

import (
	"context"
	"fmt"
	"log"

	"s3-service/saladclient"
)

func main() {
	// Create a new client
	client := saladclient.NewClient(
		"https://api.yourdomain.com",
		"your-jwt-token",
	)

	ctx := context.Background()

	// Check authentication
	authCheck, err := client.AuthCheck(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Authenticated as: %s (role: %s)\n", authCheck.Subject, authCheck.Role)
}
```

### Creating a Bucket Connection

```go
req := &saladclient.CreateBucketConnectionRequest{
	BucketName:      "my-bucket",
	Region:          "ap-southeast-2",
	RoleARN:         "arn:aws:iam::123456789:role/my-role",
	AllowedPrefixes: []string{},
}

resp, err := client.CreateBucketConnection(ctx, req)
if err != nil {
	log.Fatal(err)
}
fmt.Printf("Bucket connection created: %v\n", resp.Created)
```

### Uploading an Object

```go
data := []byte("hello world")

resp, err := client.UploadObjectWithData(
	ctx,
	"my-bucket",
	"path/to/file.txt",
	"text/plain",
	data,
	nil,
)
if err != nil {
	log.Fatal(err)
}
fmt.Printf("Uploaded: %s (ETag: %s)\n", resp.ObjectKey, resp.ETag)
```

### Downloading an Object via Presigned URL

```go
presign, err := client.PresignDownloadURL(ctx, "my-bucket", "path/to/file.txt")
if err != nil {
	log.Fatal(err)
}
fmt.Printf("Download URL: %s\n", presign.URL)
fmt.Printf("Expires at: %s\n", presign.ExpiresAt)
```

### Listing Images

```go
resp, err := client.ListImages(ctx)
if err != nil {
	log.Fatal(err)
}

for _, img := range resp.Images {
	fmt.Printf("Image: %s (%d bytes)\n", img.ObjectKey, img.SizeBytes)
}
```

### Managing Access Policies

```go
req := &saladclient.UpsertAccessPolicyRequest{
	BucketName:    "my-bucket",
	PrincipalType: "service",
	PrincipalID:   "my-service@example.com",
	Role:          "project-client",
	CanRead:       boolPtr(true),
	CanWrite:      boolPtr(true),
	CanDelete:     boolPtr(false),
	CanList:       boolPtr(true),
}

resp, err := client.UpsertAccessPolicy(ctx, req)
if err != nil {
	log.Fatal(err)
}
fmt.Printf("Policy upserted: %v\n", resp.Upserted)
```

## API Reference

### Authentication & Health

- `Health(ctx)` - Check service health
- `AuthCheck(ctx)` - Verify current authentication token

### Bucket Connections

- `CreateBucketConnection(ctx, req)` - Create a new bucket connection
- `ListBucketConnections(ctx)` - List all bucket connections

### Access Policies

- `UpsertAccessPolicy(ctx, req)` - Create or update an access policy

### Objects

- `UploadObject(ctx, req)` - Upload an object
- `UploadObjectWithData(ctx, bucket, key, contentType, data, metadata)` - Upload with raw data
- `DeleteObject(ctx, bucket, key)` - Delete an object
- `PresignUploadURL(ctx, bucket, key)` - Get a presigned URL for uploads
- `PresignDownloadURL(ctx, bucket, key)` - Get a presigned URL for downloads

### Images

- `ListImages(ctx)` - List all images
- `ListImagesByIDs(ctx, ids)` - Get specific images by ID
- `GetImage(ctx, imageID)` - Download an image
- `DeleteImage(ctx, imageID)` - Delete an image

## Error Handling

The client returns standard errors. For API-specific errors, the error message will contain the HTTP status and error code:

```go
resp, err := client.AuthCheck(ctx)
if err != nil {
	fmt.Printf("Error: %v\n", err)
	// Output: Error: API error (HTTP 401): auth_failed - authentication required
}
```

## Custom HTTP Client

You can provide a custom HTTP client for custom timeouts, proxies, or TLS configurations:

```go
httpClient := &http.Client{
	Timeout: 60 * time.Second,
}

client := saladclient.NewClientWithHTTPClient(
	"https://api.yourdomain.com",
	"your-jwt-token",
	httpClient,
)
```

## Examples

See the [examples](./examples) directory for more complete examples including:
- Complete upload/download workflows
- Batch operations
- Error handling patterns
