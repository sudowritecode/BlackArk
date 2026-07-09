export type SSEConnectionStatus = 'connected' | 'disconnected' | 'reconnecting'

export interface SSEClientEvents<T> {
  dashboard: T
  status: SSEConnectionStatus
  error: unknown
}

export type SSEListener<T> = (value: T) => void

export interface SSEClient<T = unknown> {
  on<K extends keyof SSEClientEvents<T>>(
    event: K,
    listener: SSEListener<SSEClientEvents<T>[K]>,
  ): () => void
  disconnect(): void
}

export interface ConnectSSEOptions {
  token?: string
  url?: string
  initialReconnectDelayMs?: number
  maxReconnectDelayMs?: number
}

const DEFAULT_URL = '/api/v1/events'
const DEFAULT_INITIAL_RECONNECT_DELAY_MS = 1_000
const DEFAULT_MAX_RECONNECT_DELAY_MS = 30_000

/**
 * Opens the dashboard event stream immediately. Call disconnect() when its
 * consumer unmounts; listener registration returns an unsubscribe callback.
 */
export function connectSSE<T = unknown>(options: ConnectSSEOptions = {}): SSEClient<T> {
  const listeners = new Map<keyof SSEClientEvents<T>, Set<SSEListener<never>>>()
  const initialDelay = Math.max(0, options.initialReconnectDelayMs ?? DEFAULT_INITIAL_RECONNECT_DELAY_MS)
  const maxDelay = Math.max(initialDelay, options.maxReconnectDelayMs ?? DEFAULT_MAX_RECONNECT_DELAY_MS)

  let source: EventSource | undefined
  let reconnectTimer: ReturnType<typeof setTimeout> | undefined
  let reconnectDelay = initialDelay
  let stopped = false

  const emit = <K extends keyof SSEClientEvents<T>>(event: K, value: SSEClientEvents<T>[K]) => {
    listeners.get(event)?.forEach((listener) => listener(value as never))
  }

  const eventUrl = () => {
    if (!options.token) return options.url ?? DEFAULT_URL

    const base = typeof window === 'undefined' ? 'http://localhost' : window.location.href
    const url = new URL(options.url ?? DEFAULT_URL, base)
    url.searchParams.set('token', options.token)
    return url.toString()
  }

  const open = () => {
    if (stopped) return

    source = options.token
      ? new EventSource(eventUrl())
      : createEventSource(options.url ?? DEFAULT_URL)
    source.onopen = () => {
      reconnectDelay = initialDelay
      emit('status', 'connected')
    }
    source.onmessage = (event) => {
      try {
        emit('dashboard', JSON.parse(event.data) as T)
      } catch (error) {
        emit('error', error)
      }
    }
    source.onerror = (error) => {
      source?.close()
      source = undefined
      emit('error', error)

      if (stopped || reconnectTimer) return
      emit('status', 'reconnecting')
      reconnectTimer = setTimeout(() => {
        reconnectTimer = undefined
        open()
      }, reconnectDelay)
      reconnectDelay = Math.min(reconnectDelay * 2, maxDelay)
    }
  }

  open()

  return {
    on(event, listener) {
      let eventListeners = listeners.get(event)
      if (!eventListeners) {
        eventListeners = new Set()
        listeners.set(event, eventListeners)
      }
      eventListeners.add(listener as SSEListener<never>)
      return () => eventListeners.delete(listener as SSEListener<never>)
    },
    disconnect() {
      if (stopped) return
      stopped = true
      source?.close()
      source = undefined
      if (reconnectTimer) clearTimeout(reconnectTimer)
      reconnectTimer = undefined
      emit('status', 'disconnected')
      listeners.clear()
    },
  }
}
import { createEventSource } from '../api'
