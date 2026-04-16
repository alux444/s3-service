import type {
  AccessPolicyInput,
  AccessPolicyUpsertPayload,
  ApiDataEnvelope,
  ApiError,
  ApiErrorEnvelope,
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
import type { S3ServiceApi } from './s3-service-api'
import type { HttpClient, HttpMethod } from '../http/http-client'
import type { SettingsStore } from '../runtime/app-dependencies'

export interface S3ServiceApiGatewayDependencies {
  readonly httpClient: HttpClient
  readonly settingsStore: SettingsStore
}

export interface ApiRequestError extends Error {
  readonly status: number
  readonly apiError: ApiError
}

interface JsonRequestOptions {
  readonly method: HttpMethod
  readonly path: string
  readonly signal: AbortSignal | undefined
  readonly body: unknown | undefined
}

interface ApiErrorRecord {
  readonly code?: unknown
  readonly message?: unknown
  readonly requestId?: unknown
  readonly details?: unknown
}

const isRecord = (value: unknown): value is Record<string, unknown> => {
  return typeof value === 'object' && value !== null
}

const isApiErrorEnvelope = (value: unknown): value is ApiErrorEnvelope => {
  if (!isRecord(value)) {
    return false
  }

  return isRecord(value.error)
}

const isApiDataEnvelope = <TPayload>(value: unknown): value is ApiDataEnvelope<TPayload> => {
  if (!isRecord(value)) {
    return false
  }

  return 'data' in value
}

const toApiError = (status: number, payload: unknown): ApiError => {
  if (!isApiErrorEnvelope(payload)) {
    return {
      code: 'upstream_failure',
      message: `Request failed with status ${status}`,
    }
  }

  const errorRecord: ApiErrorRecord = payload.error
  const code: string = typeof errorRecord.code === 'string' ? errorRecord.code : 'upstream_failure'
  const message: string =
    typeof errorRecord.message === 'string'
      ? errorRecord.message
      : `Request failed with status ${status}`
  const requestId: string | undefined =
    typeof errorRecord.requestId === 'string' ? errorRecord.requestId : undefined
  const apiError: ApiError = {
    code,
    message,
    details: errorRecord.details,
  }

  if (requestId !== undefined) {
    return {
      ...apiError,
      requestId,
    }
  }

  return apiError
}

const createApiRequestError = (status: number, apiError: ApiError): ApiRequestError => {
  const error: ApiRequestError = Object.assign(new Error(apiError.message), {
    status,
    apiError,
  })

  return error
}

export const isApiRequestError = (error: unknown): error is ApiRequestError => {
  if (!(error instanceof Error)) {
    return false
  }

  return 'status' in error && 'apiError' in error
}

const isSuccessStatus = (status: number): boolean => {
  return status >= 200 && status <= 299
}

const createAuthHeader = async (settingsStore: SettingsStore): Promise<string | undefined> => {
  const settings = await settingsStore.getSettings()
  if (settings === undefined) {
    return undefined
  }

  const trimmedToken: string = settings.bearerToken.trim()
  if (trimmedToken === '') {
    return undefined
  }

  return `Bearer ${trimmedToken}`
}

const createRequestHeaders = (
  authorizationHeader: string | undefined,
  hasBody: boolean,
): Record<string, string> => {
  const bodyHeaders: Array<[string, string]> = hasBody
    ? [['Content-Type', 'application/json']]
    : []
  const authorizationHeaders: Array<[string, string]> =
    authorizationHeader === undefined ? [] : [['Authorization', authorizationHeader]]
  const headerEntries: Array<[string, string]> = [
    ['Accept', 'application/json'],
    ...authorizationHeaders,
    ...bodyHeaders,
  ]

  return Object.fromEntries(headerEntries)
}

const createImageListPath = (ids: string[] | undefined): string => {
  if (ids === undefined || ids.length === 0) {
    return '/v1/images'
  }

  const encodedIds: string = ids.map((id: string): string => encodeURIComponent(id)).join(',')
  return `/v1/images?ids=${encodedIds}`
}

const requestJsonData = async <TPayload>(
  dependencies: S3ServiceApiGatewayDependencies,
  { method, path, signal, body }: JsonRequestOptions,
): Promise<ApiDataEnvelope<TPayload>> => {
  const authorizationHeader: string | undefined = await createAuthHeader(dependencies.settingsStore)
  const hasBody: boolean = body !== undefined
  const requestBody: string | undefined = hasBody ? JSON.stringify(body) : undefined
  const headers: Record<string, string> = createRequestHeaders(authorizationHeader, hasBody)
  const requestOptionsWithoutSignal = {
    method,
    path,
    headers,
  }
  const requestOptionsWithoutBody =
    signal === undefined
      ? requestOptionsWithoutSignal
      : { ...requestOptionsWithoutSignal, signal }
  const requestOptions =
    requestBody === undefined
      ? requestOptionsWithoutBody
      : { ...requestOptionsWithoutBody, body: requestBody }
  const response = await dependencies.httpClient.request<unknown>(requestOptions)

  if (isSuccessStatus(response.status) && isApiDataEnvelope<TPayload>(response.payload)) {
    return response.payload
  }

  const apiError: ApiError = toApiError(response.status, response.payload)
  throw createApiRequestError(response.status, apiError)
}

export const createS3ServiceApiGateway = (
  dependencies: S3ServiceApiGatewayDependencies,
): S3ServiceApi => {
  return {
    getHealth: (signal?: AbortSignal): Promise<ApiDataEnvelope<HealthPayload>> => {
      return requestJsonData<HealthPayload>(dependencies, {
        method: 'GET',
        path: '/health',
        signal,
        body: undefined,
      })
    },
    getAuthCheck: (signal?: AbortSignal): Promise<ApiDataEnvelope<AuthCheckPayload>> => {
      return requestJsonData<AuthCheckPayload>(dependencies, {
        method: 'GET',
        path: '/v1/auth-check',
        signal,
        body: undefined,
      })
    },
    createBucketConnection: (
      input: BucketConnectionInput,
      signal?: AbortSignal,
    ): Promise<ApiDataEnvelope<BucketConnectionCreatePayload>> => {
      return requestJsonData<BucketConnectionCreatePayload>(dependencies, {
        method: 'POST',
        path: '/v1/bucket-connections',
        body: input,
        signal,
      })
    },
    listBucketConnections: (
      signal?: AbortSignal,
    ): Promise<ApiDataEnvelope<BucketConnectionListPayload>> => {
      return requestJsonData<BucketConnectionListPayload>(dependencies, {
        method: 'GET',
        path: '/v1/bucket-connections',
        signal,
        body: undefined,
      })
    },
    upsertAccessPolicy: (
      input: AccessPolicyInput,
      signal?: AbortSignal,
    ): Promise<ApiDataEnvelope<AccessPolicyUpsertPayload>> => {
      return requestJsonData<AccessPolicyUpsertPayload>(dependencies, {
        method: 'POST',
        path: '/v1/access-policies',
        body: input,
        signal,
      })
    },
    uploadObject: (
      input: ObjectUploadInput,
      signal?: AbortSignal,
    ): Promise<ApiDataEnvelope<ObjectUploadPayload>> => {
      return requestJsonData<ObjectUploadPayload>(dependencies, {
        method: 'POST',
        path: '/v1/objects/upload',
        body: input,
        signal,
      })
    },
    deleteObject: (
      input: ObjectDeleteInput,
      signal?: AbortSignal,
    ): Promise<ApiDataEnvelope<ObjectDeletePayload>> => {
      return requestJsonData<ObjectDeletePayload>(dependencies, {
        method: 'DELETE',
        path: '/v1/objects',
        body: input,
        signal,
      })
    },
    presignUpload: (
      input: PresignUploadInput,
      signal?: AbortSignal,
    ): Promise<ApiDataEnvelope<PresignPayload>> => {
      return requestJsonData<PresignPayload>(dependencies, {
        method: 'POST',
        path: '/v1/objects/presign-upload',
        body: input,
        signal,
      })
    },
    presignDownload: (
      input: PresignDownloadInput,
      signal?: AbortSignal,
    ): Promise<ApiDataEnvelope<PresignPayload>> => {
      return requestJsonData<PresignPayload>(dependencies, {
        method: 'POST',
        path: '/v1/objects/presign-download',
        body: input,
        signal,
      })
    },
    listImages: (ids?: string[], signal?: AbortSignal): Promise<ApiDataEnvelope<ImageListPayload>> => {
      return requestJsonData<ImageListPayload>(dependencies, {
        method: 'GET',
        path: createImageListPath(ids),
        signal,
        body: undefined,
      })
    },
    deleteImageById: (
      imageId: string,
      signal?: AbortSignal,
    ): Promise<ApiDataEnvelope<ImageDeletePayload>> => {
      return requestJsonData<ImageDeletePayload>(dependencies, {
        method: 'DELETE',
        path: `/v1/images/${encodeURIComponent(imageId)}`,
        signal,
        body: undefined,
      })
    },
  }
}
