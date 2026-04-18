import { atom } from 'jotai'

export interface SetupSettingsDraft {
  readonly baseUrl: string
  readonly bearerToken: string
}

export interface SetupSettingsState {
  readonly draft: SetupSettingsDraft
  readonly isHydrated: boolean
}

export interface RequestHistoryEntry {
  readonly id: string
  readonly timestampIso: string
  readonly operationName: string
  readonly method: string
  readonly path: string
  readonly status?: number
  readonly outcome: 'succeeded' | 'failed'
  readonly requestBody?: unknown
  readonly responseBody?: unknown
  readonly errorCode?: string
  readonly errorMessage?: string
}

const initialSetupSettingsDraft: SetupSettingsDraft = {
  baseUrl: '',
  bearerToken: '',
}

export const setupSettingsStateAtom = atom<SetupSettingsState>({
  draft: initialSetupSettingsDraft,
  isHydrated: false,
})

export const requestHistoryAtom = atom<RequestHistoryEntry[]>([])

export const appendRequestHistoryAtom = atom(
  null,
  (get, set, entry: RequestHistoryEntry): void => {
    const currentEntries: RequestHistoryEntry[] = get(requestHistoryAtom)
    const nextEntries: RequestHistoryEntry[] = [entry, ...currentEntries].slice(0, 200)
    set(requestHistoryAtom, nextEntries)
  },
)

export const clearRequestHistoryAtom = atom(null, (_get, set): void => {
  set(requestHistoryAtom, [])
})
