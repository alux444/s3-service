import { useAtomValue, useSetAtom } from 'jotai'
import { type ReactElement } from 'react'
import {
  clearRequestHistoryAtom,
  requestHistoryAtom,
  type RequestHistoryEntry,
} from '../state/workbench-atoms'
import { PageFrame } from './page-frame'

const createEntryTitle = (entry: RequestHistoryEntry): string => {
  return `${entry.method} ${entry.path}`
}

export const RequestsPage = (): ReactElement => {
  const requestHistory: RequestHistoryEntry[] = useAtomValue(requestHistoryAtom)
  const clearHistory = useSetAtom(clearRequestHistoryAtom)

  return (
    <PageFrame
      title="Requests"
      description="Inspect chronological request history captured from all workflow pages."
    >
      <article className="workbench-card">
        <h2>History</h2>
        <div className="workbench-actions">
          <button type="button" onClick={(): void => clearHistory()}>Clear History</button>
        </div>
        <ul className="workbench-history-list">
          {requestHistory.map((entry: RequestHistoryEntry): ReactElement => {
            return (
              <li key={entry.id} className="workbench-history-item">
                <strong>{createEntryTitle(entry)}</strong>
                <span>{entry.timestampIso}</span>
                <span>Status: {entry.status ?? 'unknown'}</span>
                <span>Outcome: {entry.outcome}</span>
                <span>Error: {entry.errorCode ?? 'none'}</span>
              </li>
            )
          })}
        </ul>
      </article>
    </PageFrame>
  )
}
