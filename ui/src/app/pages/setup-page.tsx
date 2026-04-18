import { useAtom } from 'jotai'
import { type ChangeEvent, type ReactElement, useEffect, useState } from 'react'
import { useAppDependencies } from '../../core/runtime/use-app-dependencies'
import {
  setupSettingsStateAtom,
  type SetupSettingsDraft,
  type SetupSettingsState,
} from '../state/workbench-atoms'
import { PageFrame } from './page-frame'

const createEmptyDraft = (): SetupSettingsDraft => {
  return {
    baseUrl: '',
    bearerToken: '',
  }
}

const toHydratedState = (draft: SetupSettingsDraft): SetupSettingsState => {
  return {
    draft,
    isHydrated: true,
  }
}

const createDraftFromUnknownSettings = (
  settings: SetupSettingsDraft | undefined,
): SetupSettingsDraft => {
  if (settings === undefined) {
    return createEmptyDraft()
  }

  return settings
}

export const SetupPage = (): ReactElement => {
  const { settingsStore } = useAppDependencies()
  const [setupState, setSetupState] = useAtom(setupSettingsStateAtom)
  const [notice, setNotice] = useState<string>('No changes saved yet.')

  useEffect((): void => {
    if (setupState.isHydrated) {
      return
    }

    void settingsStore.getSettings().then((settings: SetupSettingsDraft | undefined): void => {
      const nextDraft: SetupSettingsDraft = createDraftFromUnknownSettings(settings)
      setSetupState(toHydratedState(nextDraft))
    })
  }, [setSetupState, settingsStore, setupState.isHydrated])

  const updateBaseUrl = (event: ChangeEvent<HTMLInputElement>): void => {
    const nextDraft: SetupSettingsDraft = {
      ...setupState.draft,
      baseUrl: event.target.value,
    }
    setSetupState(toHydratedState(nextDraft))
  }

  const updateBearerToken = (event: ChangeEvent<HTMLTextAreaElement>): void => {
    const nextDraft: SetupSettingsDraft = {
      ...setupState.draft,
      bearerToken: event.target.value,
    }
    setSetupState(toHydratedState(nextDraft))
  }

  const saveSettings = (): void => {
    void settingsStore.saveSettings(setupState.draft).then((): void => {
      setNotice('Settings saved to browser storage.')
    })
  }

  const clearSettings = (): void => {
    void settingsStore.clearSettings().then((): void => {
      setSetupState(toHydratedState(createEmptyDraft()))
      setNotice('Settings cleared from browser storage.')
    })
  }

  return (
    <PageFrame
      title="Setup"
      description="Configure API base URL and bearer token before running endpoint workflows."
    >
      <article className="workbench-card">
        <h2>Connection Settings</h2>
        <label className="workbench-field">
          <span>Base URL</span>
          <input
            value={setupState.draft.baseUrl}
            onChange={updateBaseUrl}
            placeholder="http://localhost:8080"
          />
        </label>
        <label className="workbench-field">
          <span>Bearer Token</span>
          <textarea
            value={setupState.draft.bearerToken}
            onChange={updateBearerToken}
            rows={6}
            placeholder="Paste JWT token"
          />
        </label>
        <div className="workbench-actions">
          <button type="button" onClick={saveSettings}>Save Settings</button>
          <button type="button" onClick={clearSettings}>Clear Settings</button>
        </div>
        <p className="workbench-notice">{notice}</p>
      </article>
    </PageFrame>
  )
}
