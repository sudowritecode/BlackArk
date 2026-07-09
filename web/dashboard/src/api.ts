import { apiToken } from './auth'

export function apiFetch(input: RequestInfo | URL, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers)

  if (apiToken.value) {
    headers.set('Authorization', `Bearer ${apiToken.value}`)
  }

  return fetch(input, { ...init, headers })
}

export function createEventSource(path: string | URL): EventSource {
  const url = new URL(path, window.location.origin)

  if (apiToken.value) {
    url.searchParams.set('token', apiToken.value)
  }

  return new EventSource(url)
}
