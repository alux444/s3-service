package handlers

import (
	"net/http"

	"s3-service/internal/httpapi"
)

// objectUploadRequest is a staging contract for task 3.4 upload implementation.
//
// WISHFUL THINKING (3.4):
// - Decide whether this should stay JSON+base64 or move to multipart/form-data.
// - For large files, multipart is preferred to avoid excessive memory pressure.
// - Validate metadata keys against an allowlist.
type objectUploadRequest struct {
	BucketName  string            `json:"bucket_name"`
	ObjectKey   string            `json:"object_key"`
	ContentType string            `json:"content_type"`
	ContentB64  string            `json:"content_b64"`
	Metadata    map[string]string `json:"metadata"`
}

// objectUploadResponse is the expected success envelope payload for upload.
//
// WISHFUL THINKING (3.4):
// - Include version_id once bucket versioning behavior is confirmed.
// - Include canonical URL pointer if/when image streaming endpoints are finalized.
type objectUploadResponse struct {
	Uploaded  bool   `json:"uploaded"`
	Bucket    string `json:"bucket"`
	ObjectKey string `json:"object_key"`
	ETag      string `json:"etag,omitempty"`
	Size      int64  `json:"size,omitempty"`
}

// writeUploadNotImplemented is a temporary helper for transitioning from placeholder upload route
// to the full service-backed upload path.
//
// WISHFUL THINKING (3.4):
// - Replace this with upload service call + error mapping.
func writeUploadNotImplemented(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, r, http.StatusNotImplemented, "not_implemented", "upload is not implemented yet", nil)
}
