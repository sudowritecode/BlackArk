<script setup lang="ts">
import { nextTick, ref, useTemplateRef } from 'vue'
import { clearApiToken, isAuthenticated, maskedApiToken, setApiToken } from '../auth'

const isOpen = ref(false)
const draftToken = ref('')
const tokenInput = useTemplateRef<HTMLInputElement>('tokenInput')

async function open(): Promise<void> {
  draftToken.value = ''
  isOpen.value = true
  await nextTick()
  tokenInput.value?.focus()
}

function close(): void {
  isOpen.value = false
  draftToken.value = ''
}

function save(): void {
  if (!draftToken.value.trim()) return
  setApiToken(draftToken.value)
  close()
}

function clear(): void {
  clearApiToken()
  close()
}
</script>

<template>
  <div class="auth-config">
    <span v-if="!isAuthenticated" class="auth-config__warning" role="status">
      <span aria-hidden="true">⚠</span> Not authenticated
    </span>
    <span v-else class="auth-config__masked" title="Configured API token">
      {{ maskedApiToken }}
    </span>

    <button class="auth-config__settings" type="button" aria-haspopup="dialog" @click="open">
      Auth settings
    </button>

    <div v-if="isOpen" class="auth-config__backdrop" @click.self="close">
      <section
        class="auth-config__dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="auth-config-title"
        @keydown.esc="close"
      >
        <h2 id="auth-config-title">API authentication</h2>
        <p>Token values are stored only in this browser.</p>

        <form @submit.prevent="save">
          <label for="api-token">API token</label>
          <input
            id="api-token"
            ref="tokenInput"
            v-model="draftToken"
            type="password"
            autocomplete="off"
            :placeholder="isAuthenticated ? maskedApiToken : 'Enter API token'"
          />

          <div class="auth-config__actions">
            <button v-if="isAuthenticated" class="auth-config__clear" type="button" @click="clear">
              Clear token
            </button>
            <span class="auth-config__spacer" />
            <button type="button" @click="close">Cancel</button>
            <button type="submit" :disabled="!draftToken.trim()">Save token</button>
          </div>
        </form>
      </section>
    </div>
  </div>
</template>

<style scoped>
.auth-config { display: flex; align-items: center; gap: .75rem; }
.auth-config__warning { color: #fbbf24; font-weight: 600; }
.auth-config__masked { color: #94a3b8; font-family: ui-monospace, monospace; }
.auth-config button { border: 1px solid #64748b; border-radius: .375rem; padding: .4rem .7rem; color: #f8fafc; background: #334155; cursor: pointer; }
.auth-config button:disabled { cursor: not-allowed; opacity: .5; }
.auth-config__backdrop { position: fixed; z-index: 100; inset: 0; display: grid; place-items: center; padding: 1rem; background: rgb(15 23 42 / 70%); }
.auth-config__dialog { width: min(28rem, 100%); padding: 1.5rem; color: #0f172a; background: white; border-radius: .75rem; box-shadow: 0 20px 40px rgb(0 0 0 / 30%); }
.auth-config__dialog h2 { margin: 0 0 .5rem; }
.auth-config__dialog p { margin: 0 0 1.25rem; color: #475569; }
.auth-config__dialog label { display: block; margin-bottom: .4rem; font-weight: 600; }
.auth-config__dialog input { box-sizing: border-box; width: 100%; padding: .65rem; border: 1px solid #94a3b8; border-radius: .375rem; }
.auth-config__actions { display: flex; gap: .5rem; margin-top: 1.25rem; }
.auth-config__actions .auth-config__clear { color: #b91c1c; background: white; border-color: #ef4444; }
.auth-config__spacer { flex: 1; }
</style>
