import { type ReactElement, useState } from 'react'
import { useApiRequestExecutor } from '../hooks/use-api-request-executor'
import { JsonPanel, PageFrame } from './page-frame'

const createEmptyPayload = (): Record<string, string> => {
  return {
    info: 'Run health or auth-check to inspect a live response.',
  }
}

export const AuthPage = (): ReactElement => {
  const { executeApiRequest } = useApiRequestExecutor()
  const [lastPayload, setLastPayload] = useState<unknown>(createEmptyPayload())

  const runHealthCheck = (): void => {
    void executeApiRequest({
      operationName: 'health_check',
      method: 'GET',
      path: '/health',
      successStatus: 200,
      execute: (api) => api.getHealth(),
    }).then((payload: unknown): void => {
      if (payload !== undefined) {
        setLastPayload(payload)
      }
    })
  }

  const runAuthCheck = (): void => {
    void executeApiRequest({
      operationName: 'auth_check',
      method: 'GET',
      path: '/v1/auth-check',
      successStatus: 200,
      execute: (api) => api.getAuthCheck(),
    }).then((payload: unknown): void => {
      if (payload !== undefined) {
        setLastPayload(payload)
      }
    })
  }

  return (
    <PageFrame
      title="Auth"
      description="Run health and auth-check endpoints to validate service availability and token claims."
    >
      <article className="workbench-card">
        <h2>Run Checks</h2>
        <div className="workbench-actions">
          <button type="button" onClick={runHealthCheck}>GET /health</button>
          <button type="button" onClick={runAuthCheck}>GET /v1/auth-check</button>
        </div>
      </article>
      <JsonPanel title="Last Response" payload={lastPayload} />
    </PageFrame>
  )
}
