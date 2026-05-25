<template>
  <div class="usage-dashboard">
    <div v-if="hasData" class="usage-summary">
      <article v-for="card in summaryCards" :key="card.label" class="usage-metric">
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
      </article>
    </div>

    <div v-if="hasData" class="usage-layout">
      <section class="usage-section">
        <div class="usage-section-head">
          <h3>{{ t('usageChart.trendTitle') }}</h3>
          <span>{{ t('usageChart.hourlyWindow') }}</span>
        </div>
        <svg class="usage-trend" viewBox="0 0 720 220" role="img" :aria-label="t('usageChart.trendTitle')">
          <line x1="44" y1="180" x2="696" y2="180" class="axis" />
          <line x1="44" y1="28" x2="44" y2="180" class="axis" />
          <g v-for="bucket in trendBuckets" :key="bucket.key">
            <rect
              class="bar-input"
              :x="bucket.x"
              :y="bucket.promptY"
              :width="bucket.width"
              :height="bucket.promptHeight"
              rx="3"
            />
            <rect
              class="bar-output"
              :x="bucket.x"
              :y="bucket.completionY"
              :width="bucket.width"
              :height="bucket.completionHeight"
              rx="3"
            />
            <rect
              class="bar-reasoning"
              :x="bucket.x"
              :y="bucket.reasoningY"
              :width="bucket.width"
              :height="bucket.reasoningHeight"
              rx="3"
            />
            <text class="bucket-label" :x="bucket.x + bucket.width / 2" y="202" text-anchor="middle">
              {{ bucket.label }}
            </text>
          </g>
          <text class="axis-label" x="44" y="18">{{ formatNumber(maxTrendTokens) }}</text>
        </svg>
        <div class="usage-legend">
          <span><i class="legend-input" />{{ t('usageChart.input') }}</span>
          <span><i class="legend-output" />{{ t('usageChart.output') }}</span>
          <span><i class="legend-reasoning" />{{ t('usageChart.reasoning') }}</span>
        </div>
      </section>

      <section class="usage-section">
        <div class="usage-section-head">
          <h3>{{ t('usageChart.modelRankTitle') }}</h3>
          <span>{{ t('usageChart.topModels', { count: modelRows.length }) }}</span>
        </div>
        <div class="model-rows">
          <article v-for="row in modelRows" :key="row.model" class="model-row">
            <div class="model-main">
              <strong>{{ row.model }}</strong>
              <span>{{ t('usageChart.modelMeta', { requests: formatNumber(row.requests), success: row.successRate }) }}</span>
            </div>
            <div class="stacked-bar" :style="{ '--total-width': `${row.width}%` }">
              <div class="stacked-fill">
                <span class="seg-input" :style="{ width: `${row.promptPercent}%` }" />
                <span class="seg-output" :style="{ width: `${row.completionPercent}%` }" />
                <span class="seg-reasoning" :style="{ width: `${row.reasoningPercent}%` }" />
              </div>
            </div>
            <div class="model-stats">
              <span>{{ formatNumber(row.totalTokens) }} token</span>
              <span>{{ t('usageChart.reasoningShort', { tokens: formatNumber(row.reasoningTokens) }) }}</span>
            </div>
          </article>
        </div>
      </section>
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
import { formatNumber } from '../utils/format'

const appStore = useAppStore()
const { t } = useI18n()

const hasData = computed(() => (appStore.stats?.total_requests ?? 0) > 0)

const successRate = computed(() => {
  const total = appStore.stats?.total_requests ?? 0
  if (!total) return 0
  return Math.round(((appStore.stats?.successful ?? 0) / total) * 1000) / 10
})

const summaryCards = computed(() => [
  { label: t('usageChart.totalTokens'), value: formatNumber(appStore.stats?.total_tokens) },
  { label: t('usageChart.reasoningTokens'), value: formatNumber(appStore.stats?.reasoning_tokens) },
  { label: t('usageChart.successRate'), value: `${successRate.value}%` },
  { label: t('usageChart.requestsTotal'), value: formatNumber(appStore.stats?.total_requests) },
])

const modelRows = computed(() => {
  const entries = Object.entries(appStore.stats?.by_model ?? {})
    .filter(([, usage]) => usage.requests > 0)
    .sort(([, left], [, right]) => right.total_tokens - left.total_tokens)
    .slice(0, 10)

  const maxTokens = Math.max(1, ...entries.map(([, usage]) => usage.total_tokens))
  return entries.map(([model, usage]) => {
    const totalTokens = usage.total_tokens || 0
    const visibleTotal = Math.max(1, usage.prompt_tokens + usage.completion_tokens + usage.reasoning_tokens)
    const successRate = usage.requests ? Math.round((usage.successes / usage.requests) * 1000) / 10 : 0
    return {
      model,
      requests: usage.requests,
      totalTokens,
      reasoningTokens: usage.reasoning_tokens,
      successRate,
      width: Math.max(6, Math.round((totalTokens / maxTokens) * 100)),
      promptPercent: Math.round((usage.prompt_tokens / visibleTotal) * 100),
      completionPercent: Math.round((usage.completion_tokens / visibleTotal) * 100),
      reasoningPercent: Math.round((usage.reasoning_tokens / visibleTotal) * 100),
    }
  })
})

const trendBuckets = computed(() => {
  const records = [...(appStore.stats?.recent ?? [])].sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime())
  const bucketMap = new Map<string, { prompt: number; completion: number; reasoning: number; total: number; time: Date }>()

  for (const record of records) {
    const date = new Date(record.time)
    if (Number.isNaN(date.getTime())) continue
    date.setMinutes(0, 0, 0)
    const key = date.toISOString()
    const bucket = bucketMap.get(key) ?? { prompt: 0, completion: 0, reasoning: 0, total: 0, time: date }
    bucket.prompt += record.prompt_tokens || 0
    bucket.completion += record.completion_tokens || 0
    bucket.reasoning += record.reasoning_tokens || 0
    bucket.total += record.total_tokens || 0
    bucketMap.set(key, bucket)
  }

  const buckets = Array.from(bucketMap.entries()).slice(-12)
  const maxTokens = Math.max(1, ...buckets.map(([, bucket]) => bucket.prompt + bucket.completion + bucket.reasoning))
  const plotLeft = 56
  const plotWidth = 620
  const plotBottom = 180
  const plotHeight = 148
  const slot = plotWidth / Math.max(1, buckets.length)
  const barWidth = Math.min(34, Math.max(14, slot * 0.56))

  return buckets.map(([key, bucket], index) => {
    const promptHeight = Math.round((bucket.prompt / maxTokens) * plotHeight)
    const completionHeight = Math.round((bucket.completion / maxTokens) * plotHeight)
    const reasoningHeight = Math.round((bucket.reasoning / maxTokens) * plotHeight)
    const x = plotLeft + index * slot + (slot - barWidth) / 2
    const promptY = plotBottom - promptHeight
    const completionY = promptY - completionHeight
    const reasoningY = completionY - reasoningHeight
    return {
      key,
      x,
      width: barWidth,
      promptHeight,
      completionHeight,
      reasoningHeight,
      promptY,
      completionY,
      reasoningY,
      label: new Intl.DateTimeFormat(undefined, { hour: '2-digit' }).format(bucket.time),
    }
  })
})

const maxTrendTokens = computed(() => {
  const totals = new Map<string, number>()
  for (const record of appStore.stats?.recent ?? []) {
    const date = new Date(record.time)
    if (Number.isNaN(date.getTime())) continue
    date.setMinutes(0, 0, 0)
    const key = date.toISOString()
    const total = (record.prompt_tokens || 0) + (record.completion_tokens || 0) + (record.reasoning_tokens || 0)
    totals.set(key, (totals.get(key) ?? 0) + total)
  }
  return Math.max(0, ...Array.from(totals.values()).slice(-12))
})
</script>

<style scoped>
.usage-dashboard {
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-5);
  width: 100%;
}

.usage-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: var(--cp-space-3);
}

.usage-metric,
.usage-section {
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-md);
  background: var(--cp-color-card);
}

.usage-metric {
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-1);
  padding: var(--cp-space-4);
}

.usage-metric span,
.usage-section-head span,
.model-main span,
.model-stats,
.usage-legend {
  font-size: var(--cp-font-size-xs);
  color: var(--cp-color-text-muted);
}

.usage-metric strong {
  font-size: var(--cp-font-size-xl);
  line-height: 1.1;
  color: var(--cp-color-text);
}

.usage-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(320px, 0.9fr);
  gap: var(--cp-space-5);
}

.usage-section {
  min-width: 0;
  padding: var(--cp-space-4);
}

.usage-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--cp-space-3);
  margin-bottom: var(--cp-space-3);
}

.usage-section-head h3 {
  margin: 0;
  font-size: var(--cp-font-size-sm);
  font-weight: 700;
  color: var(--cp-color-text);
}

.usage-trend {
  width: 100%;
  height: auto;
  display: block;
}

.axis {
  stroke: var(--cp-color-border);
  stroke-width: 1;
}

.axis-label,
.bucket-label {
  fill: var(--cp-color-text-muted);
  font-size: 11px;
}

.bar-input,
.seg-input,
.legend-input {
  fill: var(--cp-color-primary);
  background: var(--cp-color-primary);
}

.bar-output,
.seg-output,
.legend-output {
  fill: var(--cp-color-success);
  background: var(--cp-color-success);
}

.bar-reasoning,
.seg-reasoning,
.legend-reasoning {
  fill: var(--cp-color-warning);
  background: var(--cp-color-warning);
}

.usage-legend {
  display: flex;
  flex-wrap: wrap;
  gap: var(--cp-space-3);
  margin-top: var(--cp-space-2);
}

.usage-legend span {
  display: inline-flex;
  align-items: center;
  gap: var(--cp-space-1);
}

.usage-legend i {
  width: 10px;
  height: 10px;
  border-radius: 2px;
}

.model-rows {
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-3);
}

.model-row {
  display: grid;
  grid-template-columns: minmax(150px, 0.7fr) minmax(120px, 1fr) minmax(120px, 0.4fr);
  align-items: center;
  gap: var(--cp-space-3);
}

.model-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.model-main strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  font-size: var(--cp-font-size-xs);
  color: var(--cp-color-text);
}

.stacked-bar {
  height: 18px;
  border-radius: 9px;
  overflow: hidden;
  background: var(--cp-color-border);
}

.stacked-fill {
  display: flex;
  width: var(--total-width);
  min-width: 24px;
  height: 100%;
  border-radius: inherit;
  overflow: hidden;
}

.stacked-fill span {
  display: block;
  height: 100%;
}

.model-stats {
  display: flex;
  flex-direction: column;
  gap: 2px;
  text-align: right;
}

.usage-empty {
  padding: var(--cp-space-4) 0;
  text-align: center;
  font-size: var(--cp-font-size-sm);
  color: var(--cp-color-text-muted);
}

@media (max-width: 960px) {
  .usage-summary,
  .usage-layout {
    grid-template-columns: 1fr;
  }

  .model-row {
    grid-template-columns: 1fr;
    gap: var(--cp-space-2);
  }

  .model-stats {
    text-align: left;
  }
}
</style>
