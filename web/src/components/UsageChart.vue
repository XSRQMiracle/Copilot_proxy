<template>
  <div class="usage-panel">
    <div v-if="rows.length" class="usage-list">
      <article v-for="row in rows" :key="row.model" class="usage-row">
        <div class="usage-meta">
          <strong class="usage-name">{{ row.model }}</strong>
          <span class="usage-stat">{{ t('usageChart.requests', { count: row.requests, tokens: formatNumber(row.tokens) }) }}</span>
        </div>
        <div class="usage-bar" :style="{ '--bar-width': `${row.width}%` }">
          <div class="usage-fill">
            <div class="usage-success" :style="{ width: `${row.successPercent}%` }" />
            <div class="usage-failed" :style="{ width: `${row.failurePercent}%` }" />
          </div>
        </div>
      </article>
    </div>

    <div v-else class="usage-empty">
      <span>{{ t('usageChart.empty') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../i18n'
import { useAppStore } from '../stores/app'

const appStore = useAppStore()
const { t } = useI18n()

function formatNumber(value: number): string {
  return new Intl.NumberFormat('zh-CN').format(value)
}

const rows = computed(() => {
  const entries = Object.entries(appStore.stats?.by_model ?? {})
    .sort(([, a], [, b]) => b.requests - a.requests)
    .slice(0, 10)

  const maxRequests = Math.max(1, ...entries.map(([, usage]) => usage.requests))
  return entries.map(([model, usage]) => {
    const requests = usage.requests || 0
    const successPercent = requests ? Math.round((usage.successes / requests) * 100) : 0
    const failurePercent = Math.max(0, 100 - successPercent)
    return {
      model,
      requests,
      tokens: usage.total_tokens,
      width: Math.max(8, Math.round((requests / maxRequests) * 100)),
      successPercent,
      failurePercent,
    }
  })
})
</script>

<style scoped>
.usage-panel {
  width: 100%;
}

.usage-list {
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-3);
}

.usage-row {
  display: grid;
  grid-template-columns: minmax(160px, 0.3fr) 1fr;
  gap: var(--cp-space-3);
  align-items: center;
}

.usage-meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.usage-name {
  font-size: var(--cp-font-size-sm);
  font-weight: 600;
  color: var(--cp-color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.usage-stat {
  font-size: var(--cp-font-size-xs);
  color: var(--cp-color-text-muted);
}

.usage-bar {
  height: 20px;
  border-radius: 10px;
  background: var(--cp-color-border);
  overflow: hidden;
}

.usage-fill {
  width: var(--bar-width);
  height: 100%;
  display: flex;
  min-width: 40px;
  border-radius: inherit;
  overflow: hidden;
  transition: width var(--cp-transition-med);
}

.usage-success {
  background: var(--cp-color-success);
  transition: width var(--cp-transition-med);
}

.usage-failed {
  background: var(--cp-color-error);
  transition: width var(--cp-transition-med);
}

.usage-empty {
  padding: var(--cp-space-4) 0;
  text-align: center;
  font-size: var(--cp-font-size-sm);
  color: var(--cp-color-text-muted);
}

@media (max-width: 640px) {
  .usage-row {
    grid-template-columns: 1fr;
    gap: var(--cp-space-1);
  }
}
</style>
