<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'

export interface ActivityEvent {
  id?: string | number
  timestamp: string | number | Date
  type: 'deploy' | 'scale' | 'restart' | 'node_health' | string
  message: string
  healthy?: boolean
  status?: string
}

const props = defineProps<{
  events: ActivityEvent[]
}>()

const log = ref<HTMLElement | null>(null)

const displayedEvents = computed(() =>
  [...props.events]
    .sort((left, right) => eventTime(right.timestamp) - eventTime(left.timestamp))
    .slice(0, 100),
)

watch(
  () => props.events,
  async () => {
    await nextTick()
    log.value?.scrollTo({ top: 0, behavior: 'smooth' })
  },
  { deep: true },
)

function eventTime(timestamp: ActivityEvent['timestamp']): number {
  const value = timestamp instanceof Date ? timestamp.getTime() : new Date(timestamp).getTime()
  return Number.isNaN(value) ? 0 : value
}

function formatTimestamp(timestamp: ActivityEvent['timestamp']): string {
  const value = toDate(timestamp)
  return Number.isNaN(value.getTime()) ? String(timestamp) : value.toLocaleString()
}

function timestampAttribute(timestamp: ActivityEvent['timestamp']): string | undefined {
  const value = toDate(timestamp)
  return Number.isNaN(value.getTime()) ? undefined : value.toISOString()
}

function toDate(timestamp: ActivityEvent['timestamp']): Date {
  return timestamp instanceof Date ? timestamp : new Date(timestamp)
}

function badgeClass(event: ActivityEvent): string {
  if (event.type !== 'node_health') return `activity-log__badge--${event.type}`

  const status = event.status?.toLowerCase()
  const unhealthy = event.healthy === false || status === 'unhealthy' || status === 'down' || status === 'error'
  return unhealthy ? 'activity-log__badge--node-unhealthy' : 'activity-log__badge--node-healthy'
}

function eventLabel(type: string): string {
  return type.split('_').join(' ')
}
</script>

<template>
  <section class="activity-log" aria-label="Activity log">
    <header class="activity-log__header">
      <h2>Activity log</h2>
    </header>

    <div ref="log" class="activity-log__entries" aria-live="polite">
      <p v-if="displayedEvents.length === 0" class="activity-log__empty">
        No recent activity
      </p>

      <ol v-else class="activity-log__list">
        <li
          v-for="(event, index) in displayedEvents"
          :key="event.id ?? `${eventTime(event.timestamp)}-${event.type}-${index}`"
          class="activity-log__entry"
        >
          <time class="activity-log__timestamp" :datetime="timestampAttribute(event.timestamp)">
            {{ formatTimestamp(event.timestamp) }}
          </time>
          <span class="activity-log__badge" :class="badgeClass(event)">
            {{ eventLabel(event.type) }}
          </span>
          <p class="activity-log__message">{{ event.message }}</p>
        </li>
      </ol>
    </div>
  </section>
</template>

<style scoped>
.activity-log {
  display: flex;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid #293241;
  border-radius: 0.5rem;
  background: #111827;
  color: #e5e7eb;
}

.activity-log__header {
  padding: 0.875rem 1rem;
  border-bottom: 1px solid #293241;
}

.activity-log__header h2 {
  margin: 0;
  font-size: 0.875rem;
  font-weight: 600;
  letter-spacing: 0.025em;
  text-transform: uppercase;
}

.activity-log__entries {
  overflow-y: auto;
}

.activity-log__list {
  margin: 0;
  padding: 0;
  list-style: none;
}

.activity-log__entry {
  display: grid;
  grid-template-columns: minmax(10rem, auto) minmax(5rem, auto) 1fr;
  gap: 0.75rem;
  align-items: start;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid #1f2937;
}

.activity-log__entry:last-child {
  border-bottom: 0;
}

.activity-log__timestamp {
  color: #9ca3af;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.75rem;
  white-space: nowrap;
}

.activity-log__badge {
  width: fit-content;
  padding: 0.125rem 0.5rem;
  border-radius: 9999px;
  background: #374151;
  color: #e5e7eb;
  font-size: 0.6875rem;
  font-weight: 600;
  line-height: 1.25rem;
  text-transform: uppercase;
}

.activity-log__badge--deploy { background: #1d4ed8; color: #dbeafe; }
.activity-log__badge--scale { background: #a16207; color: #fef3c7; }
.activity-log__badge--restart { background: #c2410c; color: #ffedd5; }
.activity-log__badge--node-healthy { background: #15803d; color: #dcfce7; }
.activity-log__badge--node-unhealthy { background: #b91c1c; color: #fee2e2; }

.activity-log__message {
  margin: 0;
  font-size: 0.875rem;
  line-height: 1.4;
  overflow-wrap: anywhere;
}

.activity-log__empty {
  margin: 0;
  padding: 2rem 1rem;
  color: #9ca3af;
  text-align: center;
}

@media (max-width: 640px) {
  .activity-log__entry {
    grid-template-columns: 1fr auto;
  }

  .activity-log__message {
    grid-column: 1 / -1;
  }
}
</style>
