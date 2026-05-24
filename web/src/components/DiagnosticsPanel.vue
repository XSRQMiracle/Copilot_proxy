<template>
  <div class="dashboard-grid" data-testid="diagnostics-panel">
    <div class="dash-card span-full">
      <div class="dash-card-header">
        <h2>{{ t('diagnosticsPanel.connectivityTitle') }}</h2>
        <button class="dash-card-btn" :disabled="running" @click="runDiagnostics">
          {{ running ? t('diagnosticsPanel.checking') : t('diagnosticsPanel.runCheck') }}
        </button>
      </div>
      <div class="dash-card-body">
        <div v-if="connectivity.loading" class="diag-muted">{{ t('diagnosticsPanel.connecting') }}</div>
        <div v-else-if="connectivity.error" class="diag-state diag-error">{{ connectivity.error }}</div>
        <div v-else-if="connectivity.data" class="diag-list">
          <div class="diag-row">
            <span>{{ t('diagnosticsPanel.server') }}</span>
            <strong class="diag-ok">{{ t('diagnosticsPanel.reachable') }}</strong>
          </div>
          <div class="diag-row">
            <span>{{ t('diagnosticsPanel.baseUrl') }}</span>
            <code>{{ connectivity.data.base_url || t('diagnosticsPanel.notProvided') }}</code>
          </div>
          <div class="diag-row">
            <span>{{ t('diagnosticsPanel.serviceStatus') }}</span>
            <strong :class="connectivity.data.service_enabled ? 'diag-ok' : 'diag-warn'">
              {{ connectivity.data.service_enabled ? t('diagnosticsPanel.running') : t('diagnosticsPanel.paused') }}
            </strong>
          </div>
          <div class="diag-row">
            <span>{{ t('diagnosticsPanel.account') }}</span>
            <span>{{ connectivity.data.active_account || t('diagnosticsPanel.notSelected') }}</span>
          </div>
        </div>
        <div v-else class="diag-muted">{{ t('diagnosticsPanel.connectivityIdle') }}</div>
      </div>
    </div>

    <div class="dash-card">
      <div class="dash-card-header">
        <h2>{{ t('diagnosticsPanel.quotaTitle') }}</h2>
      </div>
      <div class="dash-card-body">
        <div v-if="quota.loading" class="diag-muted">{{ t('diagnosticsPanel.probing') }}</div>
        <div v-else-if="quota.error" class="diag-state diag-error">{{ quota.error }}</div>
        <div v-else-if="quota.data" class="diag-list">
          <div class="diag-row">
            <span>{{ t('diagnosticsPanel.status') }}</span>
            <strong :class="quota.data.available ? 'diag-ok' : 'diag-warn'">
              {{ quota.data.available ? t('diagnosticsPanel.available') : t('diagnosticsPanel.unavailable') }}
            </strong>
          </div>
          <div v-if="quotaMessage" class="diag-note">{{ quotaMessage }}</div>
          <div v-if="quotaSnapshotCount" class="diag-row">
            <span>{{ t('diagnosticsPanel.snapshotCount') }}</span>
            <span>{{ quotaSnapshotCount }}</span>
          </div>
        </div>
        <div v-else class="diag-muted">{{ t('diagnosticsPanel.quotaIdle') }}</div>
      </div>
    </div>

    <div class="dash-card">
      <div class="dash-card-header">
        <h2>{{ t('diagnosticsPanel.statsTitle') }}</h2>
      </div>
      <div class="dash-card-body">
        <div v-if="stats.loading" class="diag-muted">{{ t('diagnosticsPanel.loadingStats') }}</div>
        <div v-else-if="stats.error" class="diag-state diag-error">{{ stats.error }}</div>
        <div v-else-if="stats.data" class="diag-list">
          <div class="stats-strip">
            <div>
              <span>{{ t('diagnosticsPanel.totalRequests') }}</span>
              <strong>{{ formatNumber(stats.data.total_requests) }}</strong>
            </div>
            <div>
              <span>{{ t('diagnosticsPanel.successful') }}</span>
              <strong class="diag-ok">{{ formatNumber(stats.data.successful) }}</strong>
            </div>
            <div>
              <span>{{ t('diagnosticsPanel.failed') }}</span>
              <strong class="diag-error-text">{{ formatNumber(stats.data.failed) }}</strong>
            </div>
          </div>
          <div class="model-summary">
            <span>{{ t('diagnosticsPanel.modelDistribution') }}</span>
            <p>{{ modelSummary }}</p>
          </div>
        </div>
        <div v-else class="diag-muted">{{ t('diagnosticsPanel.statsIdle') }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive } from 'vue'
import { statusApi, quotaApi, statsApi, type StatusResponse, type QuotaResponse, type StatsResponse } from '../api'
import { formatNumber } from '../utils/format'
import { useI18n } from '../i18n'

const { t } = useI18n()

interface SectionState<T> {
  loading: boolean
  data: T | null
  error: string
}

const connectivity = reactive<SectionState<StatusResponse>>({ loading: false, data: null, error: '' })
const quota = reactive<SectionState<QuotaResponse>>({ loading: false, data: null, error: '' })
const stats = reactive<SectionState<StatsResponse>>({ loading: false, data: null, error: '' })

const running = computed(() => connectivity.loading || quota.loading || stats.loading)

const quotaMessage = computed(() => {
  if (!quota.data) return ''
  if (quota.data.reason === 'quota_probe_failed') return quota.data.message || 'quota_probe_failed'
  return quota.data.message || quota.data.reason || ''
})

const quotaSnapshotCount = computed(() => Object.keys(quota.data?.snapshots ?? {}).length)

const modelSummary = computed(() => {
  const entries = Object.entries(stats.data?.by_model ?? {})
    .filter(([model, usage]) => model && usage.requests > 0)
    .sort(([, left], [, right]) => right.requests - left.requests)
    .slice(0, 4)

  if (!entries.length) return t('diagnosticsPanel.noModelData')
  return entries.map(([model, usage]) => `${model} ${formatNumber(usage.requests)}${t('diagnosticsPanel.countUnit')}`).join(' · ')
})

async function loadSection<T>(state: SectionState<T>, loader: () => Promise<T>) {
  state.loading = true
  state.error = ''
  try {
    state.data = await loader()
  } catch (err) {
    state.data = null
    state.error = err instanceof Error ? err.message : t('diagnosticsPanel.checkFailed')
  } finally {
    state.loading = false
  }
}

async function runDiagnostics() {
  await Promise.all([
    loadSection(connectivity, statusApi.get),
    loadSection(quota, quotaApi.get),
    loadSection(stats, statsApi.get),
  ])
}

onMounted(() => {
  runDiagnostics().catch(() => {})
})
</script>

<style scoped>
.dashboard-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: var(--cp-space-5);
}

.span-full {
  grid-column: 1 / -1;
}

.dash-card {
  background: var(--cp-color-surface);
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-lg);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  overflow: hidden;
  transition: box-shadow var(--cp-transition-med);
}

.dash-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--cp-space-4) var(--cp-space-5) 0;
}

.dash-card-header h2 {
  margin: 0;
  font-size: var(--cp-font-size-sm);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--cp-color-text-muted);
}

.dash-card-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: var(--cp-space-1) var(--cp-space-3);
  font-size: var(--cp-font-size-xs);
  font-weight: 500;
  border: 1px solid var(--cp-color-primary);
  border-radius: var(--cp-radius-sm);
  background: transparent;
  color: var(--cp-color-primary);
  cursor: pointer;
  outline: none;
  transition: all var(--cp-transition-fast);
}

.dash-card-btn:hover:not(:disabled) {
  background: var(--cp-color-primary-soft);
}

.dash-card-btn:disabled {
  opacity: 0.6;
  cursor: wait;
}

.dash-card-body {
  padding: var(--cp-space-4) var(--cp-space-5) var(--cp-space-5);
}

.diag-list {
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-3);
}

.diag-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--cp-space-3);
  color: var(--cp-color-text-secondary);
}

.diag-row code {
  max-width: 70%;
  overflow: hidden;
  font-size: var(--cp-font-size-xs);
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--cp-color-text);
}

.diag-muted,
.diag-note {
  color: var(--cp-color-text-secondary);
}

.diag-state,
.diag-note {
  padding: var(--cp-space-3);
  border-radius: var(--cp-radius-sm);
  background: var(--cp-color-primary-soft);
}

.diag-ok {
  color: var(--cp-color-success);
}

.diag-warn {
  color: var(--cp-color-warning);
}

.diag-error,
.diag-error-text {
  color: var(--cp-color-error);
}

.diag-error {
  background: var(--cp-color-error-soft);
}

.stats-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--cp-space-3);
}

.stats-strip div {
  padding: var(--cp-space-3);
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-md);
  background: var(--cp-color-card);
}

.stats-strip span,
.model-summary span {
  display: block;
  margin-bottom: var(--cp-space-1);
  font-size: var(--cp-font-size-xs);
  color: var(--cp-color-text-muted);
}

.stats-strip strong {
  font-size: var(--cp-font-size-lg);
}

.model-summary p {
  margin: 0;
  color: var(--cp-color-text-secondary);
}

@media (max-width: 1024px) {
  .dashboard-grid {
    grid-template-columns: 1fr;
  }
}
</style>
