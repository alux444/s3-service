import { type ChangeEvent, type ReactElement, useState } from 'react'
import type { BucketConnectionInput, BucketConnectionRecord } from '../../core/api/contracts'
import { useApiRequestExecutor } from '../hooks/use-api-request-executor'
import { JsonPanel, PageFrame } from './page-frame'

const initialDraft: BucketConnectionInput = {
  bucket_name: '',
  region: '',
  role_arn: '',
  external_id: '',
  allowed_prefixes: [],
}

type BucketConnectionStringField = 'bucket_name' | 'region' | 'role_arn' | 'external_id'

const parsePrefixes = (rawValue: string): string[] => {
  return rawValue
    .split(',')
    .map((prefix: string): string => prefix.trim())
    .filter((prefix: string): boolean => prefix.length > 0)
}

const createDraftWithPrefixes = (
  draft: BucketConnectionInput,
  allowedPrefixes: string[],
): BucketConnectionInput => {
  return {
    ...draft,
    allowed_prefixes: allowedPrefixes,
  }
}

export const BucketConnectionsPage = (): ReactElement => {
  const { executeApiRequest } = useApiRequestExecutor()
  const [draft, setDraft] = useState<BucketConnectionInput>(initialDraft)
  const [prefixesText, setPrefixesText] = useState<string>('images/,uploads/private/')
  const [lastPayload, setLastPayload] = useState<unknown>({ buckets: [] as BucketConnectionRecord[] })

  const updateField = (field: BucketConnectionStringField) => {
    return (event: ChangeEvent<HTMLInputElement>): void => {
      setDraft({
        ...draft,
        [field]: event.target.value,
      })
    }
  }

  const updatePrefixes = (event: ChangeEvent<HTMLInputElement>): void => {
    setPrefixesText(event.target.value)
  }

  const submitCreate = (): void => {
    const createInput: BucketConnectionInput = createDraftWithPrefixes(draft, parsePrefixes(prefixesText))
    void executeApiRequest({
      operationName: 'create_bucket_connection',
      method: 'POST',
      path: '/v1/bucket-connections',
      successStatus: 201,
      requestBody: createInput,
      execute: (api) => api.createBucketConnection(createInput),
    }).then((payload: unknown): void => {
      if (payload !== undefined) {
        setLastPayload(payload)
      }
    })
  }

  const runList = (): void => {
    void executeApiRequest({
      operationName: 'list_bucket_connections',
      method: 'GET',
      path: '/v1/bucket-connections',
      successStatus: 200,
      execute: (api) => api.listBucketConnections(),
    }).then((payload: unknown): void => {
      if (payload !== undefined) {
        setLastPayload(payload)
      }
    })
  }

  return (
    <PageFrame
      title="Bucket Connections"
      description="Create new bucket connections and list current scoped connections."
    >
      <article className="workbench-card">
        <h2>Create Connection</h2>
        <label className="workbench-field"><span>Bucket Name</span><input value={draft.bucket_name} onChange={updateField('bucket_name')} /></label>
        <label className="workbench-field"><span>Region</span><input value={draft.region} onChange={updateField('region')} /></label>
        <label className="workbench-field"><span>Role ARN</span><input value={draft.role_arn} onChange={updateField('role_arn')} /></label>
        <label className="workbench-field"><span>External ID</span><input value={draft.external_id} onChange={updateField('external_id')} /></label>
        <label className="workbench-field"><span>Allowed Prefixes</span><input value={prefixesText} onChange={updatePrefixes} /></label>
        <div className="workbench-actions">
          <button type="button" onClick={submitCreate}>POST /v1/bucket-connections</button>
          <button type="button" onClick={runList}>GET /v1/bucket-connections</button>
        </div>
      </article>
      <JsonPanel title="Last Response" payload={lastPayload} />
    </PageFrame>
  )
}
