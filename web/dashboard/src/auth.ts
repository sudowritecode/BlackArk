import { computed, readonly, ref } from 'vue'

const TOKEN_STORAGE_KEY = 'blackark.apiToken'

function readStoredToken(): string {
  if (typeof window === 'undefined') return ''
  return window.localStorage.getItem(TOKEN_STORAGE_KEY)?.trim() ?? ''
}

const token = ref(readStoredToken())

export const apiToken = readonly(token)
export const isAuthenticated = computed(() => token.value.length > 0)
export const maskedApiToken = computed(() => {
  if (!token.value) return ''
  return `••••${token.value.slice(-4)}`
})

export function setApiToken(value: string): void {
  const nextToken = value.trim()
  token.value = nextToken

  if (nextToken) {
    window.localStorage.setItem(TOKEN_STORAGE_KEY, nextToken)
  } else {
    window.localStorage.removeItem(TOKEN_STORAGE_KEY)
  }
}

export function clearApiToken(): void {
  setApiToken('')
}
