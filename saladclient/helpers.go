package saladclient

// BoolPtr returns a pointer to a bool
func BoolPtr(b bool) *bool {
	return &b
}

// StringPtr returns a pointer to a string
func StringPtr(s string) *string {
	return &s
}

// DefaultAccessPolicy returns a default access policy request for read-only access
func DefaultAccessPolicy(bucketName, principalType, principalID string) *UpsertAccessPolicyRequest {
	return &UpsertAccessPolicyRequest{
		BucketName:      bucketName,
		PrincipalType:   principalType,
		PrincipalID:     principalID,
		Role:            "read-only-client",
		CanRead:         BoolPtr(true),
		CanWrite:        BoolPtr(false),
		CanDelete:       BoolPtr(false),
		CanList:         BoolPtr(true),
		PrefixAllowlist: []string{},
	}
}

// AdminAccessPolicy returns an access policy request with full access
func AdminAccessPolicy(bucketName, principalType, principalID string) *UpsertAccessPolicyRequest {
	return &UpsertAccessPolicyRequest{
		BucketName:      bucketName,
		PrincipalType:   principalType,
		PrincipalID:     principalID,
		Role:            "admin",
		CanRead:         BoolPtr(true),
		CanWrite:        BoolPtr(true),
		CanDelete:       BoolPtr(true),
		CanList:         BoolPtr(true),
		PrefixAllowlist: []string{},
	}
}

// WriteOnlyAccessPolicy returns an access policy request with write-only access
func WriteOnlyAccessPolicy(bucketName, principalType, principalID string) *UpsertAccessPolicyRequest {
	return &UpsertAccessPolicyRequest{
		BucketName:      bucketName,
		PrincipalType:   principalType,
		PrincipalID:     principalID,
		Role:            "project-client",
		CanRead:         BoolPtr(false),
		CanWrite:        BoolPtr(true),
		CanDelete:       BoolPtr(false),
		CanList:         BoolPtr(false),
		PrefixAllowlist: []string{},
	}
}

// ReadWriteAccessPolicy returns an access policy request with read-write access
func ReadWriteAccessPolicy(bucketName, principalType, principalID string) *UpsertAccessPolicyRequest {
	return &UpsertAccessPolicyRequest{
		BucketName:      bucketName,
		PrincipalType:   principalType,
		PrincipalID:     principalID,
		Role:            "project-client",
		CanRead:         BoolPtr(true),
		CanWrite:        BoolPtr(true),
		CanDelete:       BoolPtr(false),
		CanList:         BoolPtr(true),
		PrefixAllowlist: []string{},
	}
}
