export interface ApiError {
  readonly code: string
  readonly message: string
  readonly requestId?: string
  readonly details?: unknown
}

export interface ApiDataEnvelope<TData> {
  readonly data: TData
}

export interface ApiErrorEnvelope {
  readonly error: ApiError
}

export interface HealthPayload {
  readonly status: string
}

export interface AuthCheckPayload {
  readonly sub: string
  readonly app_id: string
  readonly project_id: string
  readonly role: string
  readonly principal_type: string
}

export interface BucketConnectionInput {
  readonly bucket_name: string
  readonly region: string
  readonly role_arn: string
  readonly external_id: string
  readonly allowed_prefixes: string[]
}

export interface BucketConnectionRecord {
  readonly bucket_name: string
  readonly region: string
  readonly role_arn: string
  readonly external_id: string
  readonly allowed_prefixes: string[]
}

export interface BucketConnectionCreatePayload {
  readonly created: boolean
}

export interface BucketConnectionListPayload {
  readonly buckets: BucketConnectionRecord[]
}

export interface AccessPolicyInput {
  readonly bucket_name: string
  readonly principal_type: string
  readonly principal_id: string
  readonly role: string
  readonly can_read?: boolean
  readonly can_write?: boolean
  readonly can_delete?: boolean
  readonly can_list?: boolean
  readonly prefix_allowlist?: string[]
}

export interface AccessPolicyUpsertPayload {
  readonly upserted: boolean
}

export interface ObjectUploadInput {
  readonly bucket_name: string
  readonly object_key: string
  readonly content_type: string
  readonly content_b64: string
  readonly metadata?: Record<string, string>
}

export interface ObjectUploadPayload {
  readonly uploaded: boolean
  readonly bucket: string
  readonly object_key: string
  readonly etag: string
  readonly size: number
}

export interface ObjectDeleteInput {
  readonly bucket_name: string
  readonly object_key: string
}

export interface ObjectDeletePayload {
  readonly deleted: boolean
  readonly bucket: string
  readonly object_key: string
}

export interface PresignUploadInput {
  readonly bucket_name: string
  readonly object_key: string
  readonly content_type: string
  readonly expires_in_seconds?: number
}

export interface PresignDownloadInput {
  readonly bucket_name: string
  readonly object_key: string
  readonly expires_in_seconds?: number
}

export interface PresignPayload {
  readonly method: string
  readonly url: string
  readonly expires_in_seconds: number
}

export interface ImageListRecord {
  readonly id: string
  readonly bucket_name: string
  readonly object_key: string
  readonly size_bytes?: number
  readonly etag?: string
  readonly last_modified?: string
  readonly url: string
}

export interface ImageListPayload {
  readonly images: ImageListRecord[]
}

export interface ImageDeletePayload {
  readonly deleted: boolean
  readonly bucket: string
  readonly object_key: string
}
