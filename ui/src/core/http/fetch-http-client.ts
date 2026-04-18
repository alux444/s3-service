import type { HttpClient, HttpRequestOptions, HttpResponse } from './http-client'

export interface FetchExecutor {
  execute(url: string, init: RequestInit): Promise<Response>
}

export interface FetchHttpClientDependencies {
  readonly baseUrl: string
  readonly fetchExecutor: FetchExecutor
}

const normalizePath = (path: string): string => {
  return path.startsWith('/') ? path : `/${path}`
}

const normalizeBaseUrl = (baseUrl: string): string => {
  return baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl
}

const resolveUrl = (baseUrl: string, path: string): string => {
  if (path.startsWith('http://') || path.startsWith('https://')) {
    return path
  }

  return `${normalizeBaseUrl(baseUrl)}${normalizePath(path)}`
}

const createRequestInit = ({
  method,
  headers,
  body,
  signal,
}: HttpRequestOptions): RequestInit => {
  const requestInit: RequestInit = { method }
  if (headers !== undefined) {
    requestInit.headers = headers
  }
  if (body !== undefined) {
    requestInit.body = body
  }
  if (signal !== undefined) {
    requestInit.signal = signal
  }

  return requestInit
}

const isJsonResponse = (response: Response): boolean => {
  const contentTypeHeader = response.headers.get('content-type')
  if (contentTypeHeader === null) {
    return false
  }

  return contentTypeHeader.toLowerCase().includes('application/json')
}

const parseResponsePayload = async <TPayload>(response: Response): Promise<TPayload> => {
  if (isJsonResponse(response)) {
    const jsonPayload: unknown = await response.json()
    return jsonPayload as TPayload
  }

  const textPayload: string = await response.text()
  return textPayload as TPayload
}

export const createFetchHttpClient = ({
  baseUrl,
  fetchExecutor,
}: FetchHttpClientDependencies): HttpClient => {
  return {
    request: async <TPayload>(options: HttpRequestOptions): Promise<HttpResponse<TPayload>> => {
      const url: string = resolveUrl(baseUrl, options.path)
      const requestInit: RequestInit = createRequestInit(options)
      const response: Response = await fetchExecutor.execute(url, requestInit)
      const payload: TPayload = await parseResponsePayload<TPayload>(response)

      return {
        status: response.status,
        headers: response.headers,
        payload,
      }
    },
  }
}
