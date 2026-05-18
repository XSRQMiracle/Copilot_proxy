<template>
  <div class="quota-panel">
    <div v-if="quota?.available && rows.length" class="quota-grid">
      <article v-for="row in rows" :key="row.key" class="quota-item">
        <div class="quota-head">
          <span class="quota-dot" :class="row.tone" />
          <strong class="quota-label">{{ row.label }}</strong>
        </div>
        <div class="quota-track" :aria-label="row.ariaLabel">
          <div class="quota-fill" :class="row.tone" :style="{ width: `${row.percent}%` }" />
        </div>
        <span class="quota-caption">{{ row.caption }}</span>
      </article>
    </div>

    <div v-else class="quota-empty">
      <span>{{ displayMessage }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../i18n'
import { useAppStore } from '../stores/app'
import type { QuotaSnapshot } from '../api'

const appStore = useAppStore()
const { t } = useI18n()

const quotaLabelKeys: Record<string, string> = {
  premium_interactions: 'quotaDisplay.premiumInteractions',
  chat: 'quotaDisplay.chat',
  completions: 'quotaDisplay.completions',
  premium_requests: 'quotaDisplay.premiumRequests',
  inline_suggestions: 'quotaDisplay.inlineSuggestions',
  chat_messages: 'quotaDisplay.chatMessages',
  messages: 'quotaDisplay.messages',
}

const quota = computed(() => appStore.quota)

const displayMessage = computed(() => {
  const q = quota.value
  if (!q) return t('quotaDisplay.quotaError')
  if (q.reason === 'quota_probe_failed') return q.message || t('quotaDisplay.quotaError')
  if (q.reason === 'quota_unrecognized') return q.message || t('quotaDisplay.quotaUnrecognized')
  return q.message || t('quotaDisplay.noQuota')
})

function snapshotPercent(snapshot: QuotaSnapshot): number {
  if (snapshot.unlimited) return 100
  const explicitPercent = snapshot.percent_remaining ?? snapshot.remaining_percent
  if (typeof explicitPercent === 'number') return clamp(explicitPercent)
  const remaining = snapshot.remaining ?? snapshot.quota_remaining ?? snapshot.remaining_quota
  const total = snapshot.entitlement ?? snapshot.limit ?? snapshot.total ?? snapshot.quota
  if (typeof remaining === 'number' && typeof total === 'number' && total > 0) {
    return clamp((remaining / total) * 100)
  }
  const used = snapshot.used ?? snapshot.consumed
  if (typeof used === 'number' && typeof total === 'number' && total > 0) {
    return clamp(((total - used) / total) * 100)
  }
  return 0
}

function clamp(value: number): number {
  return Math.max(0, Math.min(100, Math.round(value)))
}

function toneFor(percent: number): 'quota-good' | 'quota-warn' | 'quota-danger' {
  if (percent > 60) return 'quota-good'
  if (percent > 25) return 'quota-warn'
  return 'quota-danger'
}

function generatedLabel(key: string): string {
  return key
    .split(/[_\s-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

function quotaLabel(key: string): string {
  const labelKey = quotaLabelKeys[key]
  return labelKey ? t(labelKey) : generatedLabel(key)
}

function quotaKeys(snapshots: Record<string, QuotaSnapshot>): string[] {
  const known = Object.keys(quotaLabelKeys).filter((key) => snapshots[key])
  const seen = new Set(known)
  const unknown = Object.keys(snapshots).filter((key) => !seen.has(key))
  return [...known, ...unknown]
}

function quotaNumbers(snapshot: QuotaSnapshot) {
  const total = snapshot.entitlement ?? snapshot.limit ?? snapshot.total ?? snapshot.quota
  const used = snapshot.used ?? snapshot.consumed
  const remaining = snapshot.remaining ?? snapshot.quota_remaining ?? snapshot.remaining_quota
    ?? (typeof total === 'number' && typeof used === 'number' ? total - used : undefined)
  return { remaining, total, used }
}

const rows = computed(() => {
  const snapshots = appStore.quota?.snapshots ?? {}
  return quotaKeys(snapshots)
    .map((key) => {
      const snapshot = snapshots[key]
      if (!snapshot) return null
      const percent = snapshotPercent(snapshot)
      const { remaining, total, used } = quotaNumbers(snapshot)
      const label = quotaLabel(key)
      const caption = snapshot.unlimited
        ? t('quotaDisplay.unlimited')
        : t('quotaDisplay.remaining', {
            remaining: remaining ?? '-',
            total: total ?? (typeof used === 'number' ? `${t('quotaDisplay.used')} ${used}` : '-'),
            percent,
          })
      return {
        key,
        label,
        percent,
        caption,
        ariaLabel: `${label} ${caption}`,
        tone: toneFor(percent),
      }
    })
    .filter((row): row is NonNullable<typeof row> => Boolean(row))
})
</script>

<style scoped>
.quota-panel {
  width: 100%;
}

.quota-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--cp-space-4);
}

.quota-item {
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-2);
  padding: var(--cp-space-3) var(--cp-space-4);
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-md);
  background: var(--cp-color-card);
}

.quota-head {
  display: flex;
  align-items: center;
  gap: var(--cp-space-2);
}

.quota-dot {
  width: var(--cp-space-2);
  height: var(--cp-space-2);
  border-radius: 50%;
  flex-shrink: 0;
}

.quota-label {
  font-size: var(--cp-font-size-sm);
  font-weight: 600;
  color: var(--cp-color-text);
}

.quota-track {
  height: 6px;
  overflow: hidden;
  border-radius: 3px;
  background: var(--cp-color-border);
}

.quota-fill {
  height: 100%;
  border-radius: inherit;
  transition: width var(--cp-transition-med);
}

.quota-caption {
  font-size: var(--cp-font-size-xs);
  color: var(--cp-color-text-muted);
}

.quota-good { background: var(--cp-color-success); }
.quota-warn { background: var(--cp-color-warning); }
.quota-danger { background: var(--cp-color-error); }

.quota-empty {
  padding: var(--cp-space-4) 0;
  text-align: center;
  font-size: var(--cp-font-size-sm);
  color: var(--cp-color-text-muted);
}

@media (max-width: 640px) {
  .quota-grid {
    grid-template-columns: 1fr;
  }
}
</style>
