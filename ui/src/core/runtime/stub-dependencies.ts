import type {
  AccessPolicyUpsertPayload,
  ApiDataEnvelope,
  AuthCheckPayload,
  BucketConnectionCreatePayload,
  BucketConnectionListPayload,
  HealthPayload,
  ImageDeletePayload,
  ImageListPayload,
  ObjectDeletePayload,
  ObjectUploadPayload,
  PresignPayload,
} from '../api/contracts'
import type { S3ServiceApi } from '../api/s3-service-api'
import type { AppDependencies, Clock, EnvironmentSettings, SettingsStore } from './app-dependencies'

const NOT_IMPLEMENTED_MESSAGE = 'Dependency implementation not wired yet.'

const createNotImplementedError = (dependencyName: string): Error => {
  return new Error(`${dependencyName}: ${NOT_IMPLEMENTED_MESSAGE}`)
}

const rejectNotImplemented = <TPayload>(dependencyName: string): Promise<TPayload> => {
  return Promise.reject(createNotImplementedError(dependencyName))
}

const createStubS3ServiceApi = (): S3ServiceApi => {
  return {
    getHealth: (): Promise<ApiDataEnvelope<HealthPayload>> => rejectNotImplemented('s3ServiceApi.getHealth'),
    getAuthCheck: (): Promise<ApiDataEnvelope<AuthCheckPayload>> => rejectNotImplemented('s3ServiceApi.getAuthCheck'),
    createBucketConnection: (): Promise<ApiDataEnvelope<BucketConnectionCreatePayload>> => {
      return rejectNotImplemented('s3ServiceApi.createBucketConnection')
    },
    listBucketConnections: (): Promise<ApiDataEnvelope<BucketConnectionListPayload>> => {
      return rejectNotImplemented('s3ServiceApi.listBucketConnections')
    },
    upsertAccessPolicy: (): Promise<ApiDataEnvelope<AccessPolicyUpsertPayload>> => {
      return rejectNotImplemented('s3ServiceApi.upsertAccessPolicy')
    },
    uploadObject: (): Promise<ApiDataEnvelope<ObjectUploadPayload>> => rejectNotImplemented('s3ServiceApi.uploadObject'),
    deleteObject: (): Promise<ApiDataEnvelope<ObjectDeletePayload>> => rejectNotImplemented('s3ServiceApi.deleteObject'),
    presignUpload: (): Promise<ApiDataEnvelope<PresignPayload>> => rejectNotImplemented('s3ServiceApi.presignUpload'),
    presignDownload: (): Promise<ApiDataEnvelope<PresignPayload>> => rejectNotImplemented('s3ServiceApi.presignDownload'),
    listImages: (): Promise<ApiDataEnvelope<ImageListPayload>> => rejectNotImplemented('s3ServiceApi.listImages'),
    deleteImageById: (): Promise<ApiDataEnvelope<ImageDeletePayload>> => rejectNotImplemented('s3ServiceApi.deleteImageById'),
  }
}

const createStubSettingsStore = (): SettingsStore => {
  return {
    getSettings: (): Promise<EnvironmentSettings | undefined> => rejectNotImplemented('settingsStore.getSettings'),
    saveSettings: (): Promise<void> => rejectNotImplemented('settingsStore.saveSettings'),
    clearSettings: (): Promise<void> => rejectNotImplemented('settingsStore.clearSettings'),
  }
}

const createSystemClock = (): Clock => {
  return {
    now: (): Date => new Date(),
  }
}

export const createStubDependencies = (): AppDependencies => {
  return {
    s3ServiceApi: createStubS3ServiceApi(),
    settingsStore: createStubSettingsStore(),
    clock: createSystemClock(),
  }
}
