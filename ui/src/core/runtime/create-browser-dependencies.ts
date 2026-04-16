import type { AppDependencies } from './app-dependencies'
import { createS3ServiceApiGateway } from '../api/s3-service-api-gateway'
import { createFetchHttpClient, type FetchExecutor } from '../http/fetch-http-client'
import { createBrowserSettingsStore, type StorageLike } from './browser-settings-store'
import { createSystemClock } from './system-clock'

export interface CreateBrowserDependenciesOptions {
  readonly baseUrl?: string
  readonly storageKey?: string
  readonly fetchExecutor?: FetchExecutor
  readonly storage?: StorageLike
}

const DEFAULT_BASE_URL = 'http://localhost:8080'
const DEFAULT_STORAGE_KEY = 's3-service-ui.settings'

const normalizeBaseUrl = (baseUrl: string): string => {
  const trimmedBaseUrl: string = baseUrl.trim()
  return trimmedBaseUrl === '' ? DEFAULT_BASE_URL : trimmedBaseUrl
}

const resolveFetchExecutor = (fetchExecutor: FetchExecutor | undefined): FetchExecutor => {
  if (fetchExecutor !== undefined) {
    return fetchExecutor
  }

  return {
    execute: (url: string, init: RequestInit): Promise<Response> => {
      return globalThis.fetch(url, init)
    },
  }
}

const resolveStorage = (storage: StorageLike | undefined): StorageLike => {
  if (storage !== undefined) {
    return storage
  }

  return {
    getItem: (key: string): string | null => globalThis.localStorage.getItem(key),
    setItem: (key: string, value: string): void => globalThis.localStorage.setItem(key, value),
    removeItem: (key: string): void => globalThis.localStorage.removeItem(key),
  }
}

const resolveStorageKey = (storageKey: string | undefined): string => {
  if (storageKey === undefined || storageKey.trim() === '') {
    return DEFAULT_STORAGE_KEY
  }

  return storageKey.trim()
}

export const createBrowserDependencies = (
  options: CreateBrowserDependenciesOptions = {},
): AppDependencies => {
  const baseUrl: string = normalizeBaseUrl(options.baseUrl ?? DEFAULT_BASE_URL)
  const fetchExecutor: FetchExecutor = resolveFetchExecutor(options.fetchExecutor)
  const storage: StorageLike = resolveStorage(options.storage)
  const storageKey: string = resolveStorageKey(options.storageKey)
  const settingsStore = createBrowserSettingsStore({
    storage,
    storageKey,
    defaultBaseUrl: baseUrl,
  })
  const httpClient = createFetchHttpClient({
    baseUrl,
    fetchExecutor,
  })
  const s3ServiceApi = createS3ServiceApiGateway({
    httpClient,
    settingsStore,
  })

  return {
    s3ServiceApi,
    settingsStore,
    clock: createSystemClock(),
  }
}
