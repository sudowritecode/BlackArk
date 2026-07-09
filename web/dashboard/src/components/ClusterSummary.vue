<script setup lang="ts">
interface NodeCounts {
  total: number
  healthy: number
  unhealthy: number
  pending: number
}

interface AppCounts {
  total: number
  running: number
  stopped: number
  failed: number
}

defineProps<{
  nodes: NodeCounts
  apps: AppCounts
}>()
</script>

<template>
  <section class="cluster-summary" aria-label="Cluster summary">
    <article class="summary-group">
      <div class="summary-total">
        <span class="summary-value">{{ nodes.total }}</span>
        <span class="summary-label">Nodes</span>
      </div>
      <div class="summary-breakdown" aria-label="Node status counts">
        <span class="badge badge--success">{{ nodes.healthy }} healthy</span>
        <span class="badge badge--warning">{{ nodes.pending }} pending</span>
        <span class="badge badge--danger">{{ nodes.unhealthy }} unhealthy</span>
      </div>
    </article>

    <article class="summary-group">
      <div class="summary-total">
        <span class="summary-value">{{ apps.total }}</span>
        <span class="summary-label">Apps</span>
      </div>
      <div class="summary-breakdown" aria-label="App status counts">
        <span class="badge badge--success">{{ apps.running }} running</span>
        <span class="badge badge--neutral">{{ apps.stopped }} stopped</span>
        <span class="badge badge--danger">{{ apps.failed }} failed</span>
      </div>
    </article>
  </section>
</template>

<style scoped>
.cluster-summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1px;
  overflow: hidden;
  border: 1px solid #d9dee7;
  border-radius: 0.625rem;
  background: #d9dee7;
}

.summary-group {
  display: flex;
  align-items: center;
  gap: 1rem;
  min-width: 0;
  padding: 0.875rem 1rem;
  background: #fff;
}

.summary-total {
  display: flex;
  flex: 0 0 auto;
  flex-direction: column;
}

.summary-value {
  color: #111827;
  font-size: 1.5rem;
  font-weight: 700;
  line-height: 1;
}

.summary-label {
  margin-top: 0.25rem;
  color: #4b5563;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
}

.summary-breakdown {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
}

.badge {
  padding: 0.2rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  line-height: 1.25;
  white-space: nowrap;
}

.badge--success {
  color: #166534;
  background: #dcfce7;
}

.badge--warning {
  color: #854d0e;
  background: #fef9c3;
}

.badge--danger {
  color: #991b1b;
  background: #fee2e2;
}

.badge--neutral {
  color: #374151;
  background: #f3f4f6;
}

@media (max-width: 42rem) {
  .cluster-summary {
    grid-template-columns: 1fr;
  }
}
</style>
