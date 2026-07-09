<script setup lang="ts">
import { computed, ref } from 'vue'

import type { AppDetail } from '../services/api'

type SortColumn = 'name' | 'image' | 'replicas' | 'status'
type SortDirection = 'asc' | 'desc'

export interface AppsTableSort {
  column: SortColumn
  direction: SortDirection
}

const props = defineProps<{
  apps: AppDetail[]
}>()

const emit = defineEmits<{
  'sort-changed': [sort: AppsTableSort]
}>()

const sortColumn = ref<SortColumn>('name')
const sortDirection = ref<SortDirection>('asc')

const sortedApps = computed(() =>
  props.apps
    .map((app, index) => ({ app, index }))
    .sort((left, right) => {
      const comparison = compareApps(left.app, right.app, sortColumn.value)
      return (sortDirection.value === 'asc' ? comparison : -comparison) || left.index - right.index
    })
    .map(({ app }) => app),
)

function compareApps(left: AppDetail, right: AppDetail, column: SortColumn): number {
  if (column === 'replicas') {
    return (
      left.ready_replicas - right.ready_replicas ||
      left.desired_replicas - right.desired_replicas
    )
  }

  return left[column].localeCompare(right[column], undefined, {
    numeric: true,
    sensitivity: 'base',
  })
}

function changeSort(column: SortColumn): void {
  if (sortColumn.value === column) {
    sortDirection.value = sortDirection.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortColumn.value = column
    sortDirection.value = 'asc'
  }

  emit('sort-changed', { column: sortColumn.value, direction: sortDirection.value })
}

function ariaSort(column: SortColumn): 'ascending' | 'descending' | 'none' {
  if (sortColumn.value !== column) return 'none'
  return sortDirection.value === 'asc' ? 'ascending' : 'descending'
}

function sortIndicator(column: SortColumn): string {
  if (sortColumn.value !== column) return '↕'
  return sortDirection.value === 'asc' ? '↑' : '↓'
}

function normalizedStatus(status: string): 'running' | 'degraded' | 'stopped' | 'failed' {
  const value = status.toLowerCase()
  if (value === 'running' || value === 'degraded' || value === 'failed') return value
  return 'stopped'
}
</script>

<template>
  <div class="apps-table table-container">
    <table class="data-table">
      <caption class="visually-hidden">Deployed applications</caption>
      <thead>
        <tr>
          <th v-for="column in (['name', 'image', 'replicas', 'status'] as const)" :key="column" :aria-sort="ariaSort(column)">
            <button class="apps-table__sort" type="button" @click="changeSort(column)">
              <span>{{ column }}</span>
              <span class="apps-table__sort-indicator" aria-hidden="true">{{ sortIndicator(column) }}</span>
            </button>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="sortedApps.length === 0">
          <td class="apps-table__empty" colspan="4">No applications deployed</td>
        </tr>
        <tr v-for="app in sortedApps" v-else :key="app.id">
          <td class="apps-table__name">{{ app.name }}</td>
          <td><code class="apps-table__image">{{ app.image }}</code></td>
          <td>{{ app.ready_replicas }}/{{ app.desired_replicas }}</td>
          <td>
            <span class="apps-table__status" :class="`apps-table__status--${normalizedStatus(app.status)}`">
              {{ app.status }}
            </span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.apps-table__sort {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  width: 100%;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  font-weight: inherit;
  letter-spacing: inherit;
  text-align: left;
  text-transform: inherit;
  cursor: pointer;
}

.apps-table__sort-indicator {
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.apps-table__name {
  font-weight: 650;
}

.apps-table__image {
  color: #334155;
  font-size: 0.8125rem;
  overflow-wrap: anywhere;
}

.apps-table__status {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  font-weight: 600;
  text-transform: capitalize;
}

.apps-table__status::before {
  width: 0.55rem;
  height: 0.55rem;
  flex: 0 0 auto;
  border-radius: 50%;
  background: #94a3b8;
  content: '';
}

.apps-table__status--running::before { background: var(--color-success); }
.apps-table__status--degraded::before { background: var(--color-warning); }
.apps-table__status--failed::before { background: var(--color-danger); }

.apps-table__empty {
  height: 8rem;
  color: var(--color-text-muted);
  text-align: center;
}

.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
</style>
