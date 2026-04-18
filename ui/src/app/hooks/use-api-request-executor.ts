import { useCallback } from 'react'
import { useSetAtom } from 'jotai'
import type { ApiDataEnvelope } from '../../core/api/contracts'
import { isApiRequestError } from '../../core/api/s3-service-api-gateway'
import type { S3ServiceApi } from '../../core/api/s3-service-api'
import { useAppDependencies } from '../../core/runtime/use-app-dependencies'
import { appendRequestHistoryAtom, type RequestHistoryEntry } from '../state/workbench-atoms'

interface ExecuteApiRequestOptions<TPayload> {
  readonly operationName: string
  readonly method: string
  readonly path: string
  readonly successStatus: number
  readonly requestBody?: unknown
  readonly execute: (api: S3ServiceApi) => Promise<ApiDataEnvelope<TPayload>>
}

export interface ApiRequestExecutor {
  executeApiRequest: <TPayload>(options: ExecuteApiRequestOptions<TPayload>) => Promise<TPayload | undefined>
}

const createEntryId = (): string => {
  const randomPart: string = Math.random().toString(36).slice(2, 10)
  return `${Date.now()}-${randomPart}`
}

const createSuccessHistoryEntry = <TPayload>(
  options: ExecuteApiRequestOptions<TPayload>,
  response: TPayload,
): RequestHistoryEntry => {
  const baseEntry: RequestHistoryEntry = {
    id: createEntryId(),
    timestampIso: new Date().toISOString(),
    operationName: options.operationName,
    method: options.method,
    path: options.path,
    status: options.successStatus,
    outcome: 'succeeded',
    responseBody: response,
  }

  if (options.requestBody === undefined) {
    return baseEntry
  }

  return {
    ...baseEntry,
    requestBody: options.requestBody,
  }
}

const createFailureHistoryEntry = (
  options: ExecuteApiRequestOptions<unknown>,
  error: unknown,
): RequestHistoryEntry => {
  if (isApiRequestError(error)) {
    const apiErrorEntry: RequestHistoryEntry = {
      id: createEntryId(),
      timestampIso: new Date().toISOString(),
      operationName: options.operationName,
      method: options.method,
      path: options.path,
      status: error.status,
      outcome: 'failed',
      errorCode: error.apiError.code,
      errorMessage: error.apiError.message,
    }

    const withRequestBody: RequestHistoryEntry =
      options.requestBody === undefined
        ? apiErrorEntry
        : { ...apiErrorEntry, requestBody: options.requestBody }

    if (error.apiError.details === undefined) {
      return withRequestBody
    }

    return {
      ...withRequestBody,
      responseBody: error.apiError.details,
    }
  }

  const errorMessage: string = error instanceof Error ? error.message : 'Unexpected request error.'
  const fallbackEntry: RequestHistoryEntry = {
    id: createEntryId(),
    timestampIso: new Date().toISOString(),
    operationName: options.operationName,
    method: options.method,
    path: options.path,
    outcome: 'failed',
    errorCode: 'unknown_error',
    errorMessage,
  }

  if (options.requestBody === undefined) {
    return fallbackEntry
  }

  return {
    ...fallbackEntry,
    requestBody: options.requestBody,
  }
}

export const useApiRequestExecutor = (): ApiRequestExecutor => {
  const { s3ServiceApi } = useAppDependencies()
  const appendRequestHistory = useSetAtom(appendRequestHistoryAtom)

  const executeApiRequest = useCallback(
    async <TPayload>(options: ExecuteApiRequestOptions<TPayload>): Promise<TPayload | undefined> => {
      try {
        const responseEnvelope: ApiDataEnvelope<TPayload> = await options.execute(s3ServiceApi)
        appendRequestHistory(createSuccessHistoryEntry(options, responseEnvelope.data))
        return responseEnvelope.data
      } catch (error: unknown) {
        appendRequestHistory(createFailureHistoryEntry(options, error))
        return undefined
      }
    },
    [appendRequestHistory, s3ServiceApi],
  )

  return { executeApiRequest }
}
