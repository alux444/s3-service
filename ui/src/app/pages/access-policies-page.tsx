import { type ChangeEvent, type ReactElement, useState } from 'react'
import type { AccessPolicyInput } from '../../core/api/contracts'
import { useApiRequestExecutor } from '../hooks/use-api-request-executor'
import { JsonPanel, PageFrame } from './page-frame'

const initialDraft: AccessPolicyInput = {
  bucket_name: '',
  principal_type: 'service',
  principal_id: '',
  role: 'admin',
  can_read: true,
  can_write: false,
  can_delete: false,
  can_list: true,
  prefix_allowlist: [],
}

type AccessPolicyStringField = 'bucket_name' | 'principal_type' | 'principal_id' | 'role'

const parseAllowList = (rawValue: string): string[] => {
  return rawValue
    .split(',')
    .map((value: string): string => value.trim())
    .filter((value: string): boolean => value.length > 0)
}

export const AccessPoliciesPage = (): ReactElement => {
  const { executeApiRequest } = useApiRequestExecutor()
  const [draft, setDraft] = useState<AccessPolicyInput>(initialDraft)
  const [prefixAllowList, setPrefixAllowList] = useState<string>('uploads/,images/')
  const [lastPayload, setLastPayload] = useState<unknown>({ upserted: false })

  const updateField = (field: AccessPolicyStringField) => {
    return (event: ChangeEvent<HTMLInputElement>): void => {
      setDraft({
        ...draft,
        [field]: event.target.value,
      })
    }
  }

  const updateAllowList = (event: ChangeEvent<HTMLInputElement>): void => {
    setPrefixAllowList(event.target.value)
  }

  const submitUpsert = (): void => {
    const requestBody: AccessPolicyInput = {
      ...draft,
      prefix_allowlist: parseAllowList(prefixAllowList),
    }

    void executeApiRequest({
      operationName: 'upsert_access_policy',
      method: 'POST',
      path: '/v1/access-policies',
      successStatus: 200,
      requestBody,
      execute: (api) => api.upsertAccessPolicy(requestBody),
    }).then((payload: unknown): void => {
      if (payload !== undefined) {
        setLastPayload(payload)
      }
    })
  }

  return (
    <PageFrame title="Access Policies" description="Upsert principal policy permissions for a scoped bucket.">
      <article className="workbench-card">
        <h2>Policy Input</h2>
        <label className="workbench-field"><span>Bucket Name</span><input value={draft.bucket_name} onChange={updateField('bucket_name')} /></label>
        <label className="workbench-field"><span>Principal Type</span><input value={draft.principal_type} onChange={updateField('principal_type')} /></label>
        <label className="workbench-field"><span>Principal ID</span><input value={draft.principal_id} onChange={updateField('principal_id')} /></label>
        <label className="workbench-field"><span>Role</span><input value={draft.role} onChange={updateField('role')} /></label>
        <label className="workbench-field"><span>Prefix Allowlist</span><input value={prefixAllowList} onChange={updateAllowList} /></label>
        <div className="workbench-actions">
          <button type="button" onClick={submitUpsert}>POST /v1/access-policies</button>
        </div>
      </article>
      <JsonPanel title="Last Response" payload={lastPayload} />
    </PageFrame>
  )
}
