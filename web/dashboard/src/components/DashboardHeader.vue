<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AuthConfig from './AuthConfig.vue'

type DashboardStatus = 'healthy' | 'unhealthy'

const props = defineProps<{
  clusterUrl: string
  status: DashboardStatus
  lastUpdated: Date | string | number
}>()

const currentTimestamp = ref(new Date(props.lastUpdated))
let clock: ReturnType<typeof setInterval> | undefined

const isHealthy = computed(() => props.status === 'healthy')
const statusLabel = computed(() => (isHealthy.value ? 'Healthy' : 'Unhealthy'))
const formattedTimestamp = computed(() => {
  if (Number.isNaN(currentTimestamp.value.getTime())) return 'Unknown'

  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'medium',
  }).format(currentTimestamp.value)
})
const timestampIso = computed(() =>
  Number.isNaN(currentTimestamp.value.getTime()) ? undefined : currentTimestamp.value.toISOString(),
)

watch(
  () => props.lastUpdated,
  (lastUpdated) => {
    currentTimestamp.value = new Date(lastUpdated)
  },
)

onMounted(() => {
  clock = setInterval(() => {
    currentTimestamp.value = new Date()
  }, 1_000)
})

onBeforeUnmount(() => {
  if (clock) clearInterval(clock)
})
</script>

<template>
  <header class="dashboard-header">
    <h1 class="dashboard-header__title">BlackArk Dashboard</h1>

    <div class="dashboard-header__details">
      <div
        class="dashboard-header__status"
        role="status"
        :aria-label="`Cluster health: ${statusLabel}`"
      >
        <span
          class="dashboard-header__status-dot"
          :class="{ 'dashboard-header__status-dot--healthy': isHealthy }"
          aria-hidden="true"
        />
        <span>{{ statusLabel }}</span>
      </div>

      <span class="dashboard-header__cluster" :title="clusterUrl">
        {{ clusterUrl }}
      </span>

      <time :datetime="timestampIso">
        {{ formattedTimestamp }}
      </time>

      <AuthConfig />
    </div>
  </header>
</template>

<style scoped>
.dashboard-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 2rem;
  min-height: 4rem;
  padding: 0.75rem 1.5rem;
  color: #f8fafc;
  background: #0f172a;
  border-bottom: 1px solid #334155;
}

.dashboard-header__title {
  flex: none;
  margin: 0;
  font-size: 1.25rem;
  line-height: 1.5;
}

.dashboard-header__details {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 1.25rem;
  min-width: 0;
  color: #cbd5e1;
  font-size: 0.875rem;
}

.dashboard-header__status {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: #f8fafc;
  font-weight: 600;
}

.dashboard-header__status-dot {
  width: 0.625rem;
  height: 0.625rem;
  background: #ef4444;
  border-radius: 50%;
  box-shadow: 0 0 0 3px rgb(239 68 68 / 20%);
}

.dashboard-header__status-dot--healthy {
  background: #22c55e;
  box-shadow: 0 0 0 3px rgb(34 197 94 / 20%);
}

.dashboard-header__cluster {
  overflow: hidden;
  max-width: 24rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

time {
  flex: none;
  font-variant-numeric: tabular-nums;
}

@media (max-width: 900px) {
  .dashboard-header {
    align-items: flex-start;
    flex-direction: column;
    gap: 0.5rem;
  }

  .dashboard-header__details {
    justify-content: space-between;
    width: 100%;
  }
}
</style>
