import type { S3ServiceApi } from '../api/s3-service-api'

export interface EnvironmentSettings {
  readonly baseUrl: string
  readonly bearerToken: string
}

export interface SettingsStore {
  getSettings(): Promise<EnvironmentSettings | undefined>
  saveSettings(settings: EnvironmentSettings): Promise<void>
  clearSettings(): Promise<void>
}

export interface Clock {
  now(): Date
}

export interface AppDependencies {
  readonly s3ServiceApi: S3ServiceApi
  readonly settingsStore: SettingsStore
  readonly clock: Clock
}
