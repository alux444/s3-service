import type { EnvironmentSettings, SettingsStore } from './app-dependencies'

export interface StorageLike {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}

export interface BrowserSettingsStoreDependencies {
  readonly storage: StorageLike
  readonly storageKey: string
  readonly defaultBaseUrl: string
}

interface PersistedSettingsRecord {
  readonly baseUrl: string
  readonly bearerToken: string
}

const isRecord = (value: unknown): value is Record<string, unknown> => {
  return typeof value === 'object' && value !== null
}

const readStringProperty = (record: Record<string, unknown>, key: string): string | undefined => {
  const value: unknown = record[key]
  return typeof value === 'string' ? value : undefined
}

const parseStoredSettings = (serializedSettings: string): EnvironmentSettings | undefined => {
  const parsedValue: unknown = JSON.parse(serializedSettings)
  if (!isRecord(parsedValue)) {
    return undefined
  }

  const baseUrl: string | undefined = readStringProperty(parsedValue, 'baseUrl')
  const bearerToken: string | undefined = readStringProperty(parsedValue, 'bearerToken')
  if (baseUrl === undefined || bearerToken === undefined) {
    return undefined
  }

  return {
    baseUrl,
    bearerToken,
  }
}

const serializeSettings = (settings: EnvironmentSettings): string => {
  const persistedSettings: PersistedSettingsRecord = {
    baseUrl: settings.baseUrl,
    bearerToken: settings.bearerToken,
  }

  return JSON.stringify(persistedSettings)
}

const createDefaultSettings = (defaultBaseUrl: string): EnvironmentSettings | undefined => {
  if (defaultBaseUrl.trim() === '') {
    return undefined
  }

  return {
    baseUrl: defaultBaseUrl,
    bearerToken: '',
  }
}

export const createBrowserSettingsStore = ({
  storage,
  storageKey,
  defaultBaseUrl,
}: BrowserSettingsStoreDependencies): SettingsStore => {
  return {
    getSettings: async (): Promise<EnvironmentSettings | undefined> => {
      const serializedSettings: string | null = storage.getItem(storageKey)
      if (serializedSettings === null) {
        return createDefaultSettings(defaultBaseUrl)
      }

      try {
        const parsedSettings: EnvironmentSettings | undefined = parseStoredSettings(serializedSettings)
        return parsedSettings ?? createDefaultSettings(defaultBaseUrl)
      } catch {
        return createDefaultSettings(defaultBaseUrl)
      }
    },
    saveSettings: async (settings: EnvironmentSettings): Promise<void> => {
      const serializedSettings: string = serializeSettings(settings)
      storage.setItem(storageKey, serializedSettings)
    },
    clearSettings: async (): Promise<void> => {
      storage.removeItem(storageKey)
    },
  }
}
