import { type ChangeEvent, type ReactElement, useState } from 'react'
import type { S3ServiceApi } from '../../core/api/s3-service-api'
import type { ImageListRecord } from '../../core/api/contracts'
import { useApiRequestExecutor } from '../hooks/use-api-request-executor'
import { JsonPanel, PageFrame } from './page-frame'

const parseIds = (rawValue: string): string[] => {
  return rawValue
    .split(',')
    .map((value: string): string => value.trim())
    .filter((value: string): boolean => value.length > 0)
}

export const ImagesPage = (): ReactElement => {
  const { executeApiRequest } = useApiRequestExecutor()
  const [idsText, setIdsText] = useState<string>('')
  const [deleteId, setDeleteId] = useState<string>('')
  const [lastPayload, setLastPayload] = useState<unknown>({ images: [] as ImageListRecord[] })

  const runListImages = (): void => {
    const ids: string[] = parseIds(idsText)
    const hasIds: boolean = ids.length > 0
    const path: string = hasIds ? '/v1/images?ids=...' : '/v1/images'
    const optionsWithoutRequestBody: {
      readonly operationName: string
      readonly method: string
      readonly path: string
      readonly successStatus: number
      readonly execute: (api: S3ServiceApi) => Promise<{ readonly data: unknown }>
    } = {
      operationName: 'list_images',
      method: 'GET',
      path,
      successStatus: 200,
      execute: (api: S3ServiceApi): Promise<{ readonly data: unknown }> => {
        return api.listImages(hasIds ? ids : undefined)
      },
    }
    const requestOptions = hasIds
      ? { ...optionsWithoutRequestBody, requestBody: { ids } }
      : optionsWithoutRequestBody

    void executeApiRequest(requestOptions).then((payload: unknown): void => {
      if (payload !== undefined) {
        setLastPayload(payload)
      }
    })
  }

  const runDeleteById = (): void => {
    void executeApiRequest({
      operationName: 'delete_image_by_id',
      method: 'DELETE',
      path: `/v1/images/${deleteId}`,
      successStatus: 200,
      execute: (api) => api.deleteImageById(deleteId),
    }).then((payload: unknown): void => {
      if (payload !== undefined) {
        setLastPayload(payload)
      }
    })
  }

  const updateIdsText = (event: ChangeEvent<HTMLInputElement>): void => {
    setIdsText(event.target.value)
  }

  const updateDeleteId = (event: ChangeEvent<HTMLInputElement>): void => {
    setDeleteId(event.target.value)
  }

  return (
    <PageFrame title="Images" description="List image records and delete by image id.">
      <article className="workbench-card">
        <h2>Image Operations</h2>
        <label className="workbench-field"><span>IDs (comma-separated)</span><input value={idsText} onChange={updateIdsText} /></label>
        <label className="workbench-field"><span>Delete Image ID</span><input value={deleteId} onChange={updateDeleteId} /></label>
        <div className="workbench-actions">
          <button type="button" onClick={runListImages}>GET /v1/images</button>
          <button type="button" onClick={runDeleteById}>DELETE /v1/images/{'{id}'}</button>
        </div>
      </article>
      <JsonPanel title="Last Response" payload={lastPayload} />
    </PageFrame>
  )
}
