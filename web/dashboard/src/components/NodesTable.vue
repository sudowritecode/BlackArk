<script setup lang="ts">
import { computed, ref } from 'vue'

import type { NodeDetail } from '../services/api'

export type NodeSortKey = 'name' | 'status' | 'cpu' | 'memory' | 'app_count'
export type SortDirection = 'asc' | 'desc'

export interface NodeSortChange {
  key: NodeSortKey
  direction: SortDirection
}

const props = defineProps<{
  nodes: NodeDetail[]
}>()

const emit = defineEmits<{
  'sort-changed': [sort: NodeSortChange]
}>()

const sortKey = ref<NodeSortKey>('name')
const sortDirection = ref<SortDirection>('asc')

const columns: Array<{ key: NodeSortKey; label: string }> = [
  { key: 'name', label: 'Name' },
  { key: 'status', label: 'Status' },
  { key: 'cpu', label: 'CPU' },
  { key: 'memory', label: 'Memory' },
  { key: 'app_count', label: 'App count' },
]

const sortedNodes = computed(() =>
  [...props.nodes].sort((left, right) => {
    const result = compare(sortValue(left, sortKey.value), sortValue(right, sortKey.value))
    return sortDirection.value === 'asc' ? result : -result
  }),
)

function changeSort(key: NodeSortKey): void {
  if (sortKey.value === key) {
    sortDirection.value = sortDirection.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortKey.value = key
    sortDirection.value = 'asc'
  }

  emit('sort-changed', { key: sortKey.value, direction: sortDirection.value })
}

function sortValue(node: NodeDetail, key: NodeSortKey): string | number {
  switch (key) {
    case 'cpu':
      return node.cpus_used
    case 'memory':
      return node.mem_used_bytes
    case 'app_count':
      return node.app_count
    default:
      return node[key].toLocaleLowerCase()
  }
}

function compare(left: string | number, right: string | number): number {
  if (typeof left === 'number' && typeof right === 'number') return left - right
  return String(left).localeCompare(String(right), undefined, { numeric: true, sensitivity: 'base' })
}

function ariaSort(key: NodeSortKey): 'ascending' | 'descending' | 'none' {
  if (sortKey.value !== key) return 'none'
  return sortDirection.value === 'asc' ? 'ascending' : 'descending'
}

function formatCpu(value: number): string {
  return Number.isInteger(value) ? String(value) : value.toFixed(1)
}

function formatMemory(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 MB'

  const gigabyte = 1024 ** 3
  const megabyte = 1024 ** 2
  if (bytes >= gigabyte) return `${formatAmount(bytes / gigabyte)} GB`
  return `${formatAmount(bytes / megabyte)} MB`
}

function formatAmount(value: number): string {
  return value >= 10 || Number.isInteger(value) ? value.toFixed(0) : value.toFixed(1)
}

function statusClass(status: string): string {
  const normalized = status.toLocaleLowerCase()
  if (normalized === 'healthy') return 'nodes-table__status--healthy'
  if (normalized === 'pending') return 'nodes-table__status--pending'
  return 'nodes-table__status--unhealthy'
}
</script>

<template>
  <div class="table-container nodes-table">
    <table class="data-table">
      <caption class="nodes-table__caption">Cluster nodes and resource usage</caption>
      <thead>
        <tr>
          <th v-for="column in columns" :key="column.key" scope="col" :aria-sort="ariaSort(column.key)">
            <button class="nodes-table__sort" type="button" @click="changeSort(column.key)">
              {{ column.label }}
              <span class="nodes-table__sort-icon" aria-hidden="true">
                {{ sortKey === column.key ? (sortDirection === 'asc' ? '▲' : '▼') : '↕' }}
              </span>
            </button>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="sortedNodes.length === 0">
          <td class="nodes-table__empty" :colspan="columns.length">No nodes found</td>
        </tr>
        <tr v-for="node in sortedNodes" v-else :key="node.id">
          <td class="nodes-table__name">{{ node.name }}</td>
          <td>
            <span class="nodes-table__status" :class="statusClass(node.status)">
              {{ node.status }}
            </span>
          </td>
          <td>{{ formatCpu(node.cpus_used) }} / {{ formatCpu(node.cpus) }}</td>
          <td>{{ formatMemory(node.mem_used_bytes) }} / {{ formatMemory(node.mem_bytes) }}</td>
          <td>{{ node.app_count }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.nodes-table__caption {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.nodes-table__sort {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  width: 100%;
  padding: 0;
  color: inherit;
  background: transparent;
  border: 0;
  cursor: pointer;
  font: inherit;
  font-weight: inherit;
  letter-spacing: inherit;
  text-align: left;
  text-transform: inherit;
}

.nodes-table__sort-icon {
  color: var(--color-text-muted);
  font-size: 0.625rem;
}

.nodes-table__name {
  font-weight: 650;
}

.nodes-table__status {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  text-transform: capitalize;
}

.nodes-table__status::before {
  width: 0.625rem;
  height: 0.625rem;
  flex: 0 0 auto;
  border-radius: 50%;
  background: currentcolor;
  content: '';
}

.nodes-table__status--healthy {
  color: var(--color-success);
}

.nodes-table__status--pending {
  color: var(--color-warning);
}

.nodes-table__status--unhealthy {
  color: var(--color-danger);
}

.nodes-table__empty {
  height: 8rem;
  color: var(--color-text-muted);
  text-align: center;
}
</style>
