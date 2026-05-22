<template>
  <div class="cards-grid">
    <article v-for="card in cards" :key="card.label" class="metric-card">
      <span class="metric-label">{{ card.label }}</span>
      <strong class="metric-value">{{ card.value }}</strong>
      <span class="metric-meta" :class="`meta-${card.type}`">{{ card.meta }}</span>
    </article>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../i18n'
import { useAppStore } from '../stores/app'
import { formatNumber } from '../utils/format'

const appStore = useAppStore()
const { t } = useI18n()

const successRate = computed(() => {
  const total = appStore.stats?.total_requests ?? 0
  if (!total) return 0
  return Math.round(((appStore.stats?.successful ?? 0) / total) * 1000) / 10
})

const cards = computed(() => [
  {
    label: t('dashboardCards.totalRequests'),
    value: formatNumber(appStore.stats?.total_requests),
    meta: t('dashboardCards.successRate', { rate: successRate.value }),
    type: successRate.value >= 90 ? 'success' : 'warning',
  },
  {
    label: t('dashboardCards.successful'),
    value: formatNumber(appStore.stats?.successful),
    meta: t('dashboardCards.successfulMeta'),
    type: 'success',
  },
  {
    label: t('dashboardCards.failed'),
    value: formatNumber(appStore.stats?.failed),
    meta: (appStore.stats?.failed ?? 0) > 0 ? t('dashboardCards.failedMetaAttention') : t('dashboardCards.failedMetaNormal'),
    type: (appStore.stats?.failed ?? 0) > 0 ? 'error' : 'success',
  },
  {
    label: t('dashboardCards.token'),
    value: formatNumber(appStore.stats?.total_tokens),
    meta: t('dashboardCards.tokenMeta', {
      prompt: formatNumber(appStore.stats?.prompt_tokens),
      completion: formatNumber(appStore.stats?.completion_tokens),
    }),
    type: 'info',
  },
])
</script>

<style scoped>
.cards-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--cp-space-3);
}

.metric-card {
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-1);
  padding: var(--cp-space-4);
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-md);
  background: var(--cp-color-card);
  transition: all var(--cp-transition-fast);
}

.metric-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--cp-shadow-float);
}

.metric-label {
  font-size: var(--cp-font-size-xs);
  font-weight: 500;
  color: var(--cp-color-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.metric-value {
  font-size: clamp(var(--cp-font-size-xl), 2vw, 28px);
  font-weight: 700;
  color: var(--cp-color-text);
  line-height: 1.1;
}

.metric-meta {
  font-size: var(--cp-font-size-xs);
  color: var(--cp-color-text-muted);
}

.meta-success { color: var(--cp-color-success); }
.meta-warning { color: var(--cp-color-warning); }
.meta-error { color: var(--cp-color-error); }
.meta-info { color: var(--cp-color-text-muted); }

@media (max-width: 640px) {
  .cards-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
