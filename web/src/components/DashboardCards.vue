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
import { useAppStore } from '../stores/app'

const appStore = useAppStore()

function formatNumber(value: number | undefined): string {
  return new Intl.NumberFormat('zh-CN').format(value ?? 0)
}

const successRate = computed(() => {
  const total = appStore.stats?.total_requests ?? 0
  if (!total) return 0
  return Math.round(((appStore.stats?.successful ?? 0) / total) * 1000) / 10
})

const cards = computed(() => [
  {
    label: '总请求数',
    value: formatNumber(appStore.stats?.total_requests),
    meta: `成功率 ${successRate.value}%`,
    type: successRate.value >= 90 ? 'success' : 'warning',
  },
  {
    label: '成功请求',
    value: formatNumber(appStore.stats?.successful),
    meta: '已正常完成',
    type: 'success',
  },
  {
    label: '失败请求',
    value: formatNumber(appStore.stats?.failed),
    meta: (appStore.stats?.failed ?? 0) > 0 ? '需要关注' : '无异常',
    type: (appStore.stats?.failed ?? 0) > 0 ? 'error' : 'success',
  },
  {
    label: 'Token',
    value: formatNumber(appStore.stats?.total_tokens),
    meta: `输入 ${formatNumber(appStore.stats?.prompt_tokens)} / 输出 ${formatNumber(appStore.stats?.completion_tokens)}`,
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
