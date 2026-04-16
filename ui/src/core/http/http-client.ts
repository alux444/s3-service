export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

export interface HttpRequestOptions {
  readonly method: HttpMethod
  readonly path: string
  readonly headers?: Record<string, string>
  readonly body?: string
  readonly signal?: AbortSignal
}

export interface HttpResponse<TPayload> {
  readonly status: number
  readonly headers: Headers
  readonly payload: TPayload
}

export interface HttpClient {
  request<TPayload>(options: HttpRequestOptions): Promise<HttpResponse<TPayload>>
}
