import type {
  AccessPolicyInput,
  AccessPolicyUpsertPayload,
  ApiDataEnvelope,
  AuthCheckPayload,
  BucketConnectionCreatePayload,
  BucketConnectionInput,
  BucketConnectionListPayload,
  HealthPayload,
  ImageDeletePayload,
  ImageListPayload,
  ObjectDeleteInput,
  ObjectDeletePayload,
  ObjectUploadInput,
  ObjectUploadPayload,
  PresignDownloadInput,
  PresignPayload,
  PresignUploadInput,
} from './contracts'

export interface S3ServiceApi {
  getHealth(signal?: AbortSignal): Promise<ApiDataEnvelope<HealthPayload>>
  getAuthCheck(signal?: AbortSignal): Promise<ApiDataEnvelope<AuthCheckPayload>>
  createBucketConnection(
    input: BucketConnectionInput,
    signal?: AbortSignal,
  ): Promise<ApiDataEnvelope<BucketConnectionCreatePayload>>
  listBucketConnections(
    signal?: AbortSignal,
  ): Promise<ApiDataEnvelope<BucketConnectionListPayload>>
  upsertAccessPolicy(
    input: AccessPolicyInput,
    signal?: AbortSignal,
  ): Promise<ApiDataEnvelope<AccessPolicyUpsertPayload>>
  uploadObject(
    input: ObjectUploadInput,
    signal?: AbortSignal,
  ): Promise<ApiDataEnvelope<ObjectUploadPayload>>
  deleteObject(
    input: ObjectDeleteInput,
    signal?: AbortSignal,
  ): Promise<ApiDataEnvelope<ObjectDeletePayload>>
  presignUpload(
    input: PresignUploadInput,
    signal?: AbortSignal,
  ): Promise<ApiDataEnvelope<PresignPayload>>
  presignDownload(
    input: PresignDownloadInput,
    signal?: AbortSignal,
  ): Promise<ApiDataEnvelope<PresignPayload>>
  listImages(ids?: string[], signal?: AbortSignal): Promise<ApiDataEnvelope<ImageListPayload>>
  deleteImageById(
    imageId: string,
    signal?: AbortSignal,
  ): Promise<ApiDataEnvelope<ImageDeletePayload>>
}
