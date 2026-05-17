<template>
  <div class="quota-panel">
    <div v-if="quota?.available && rows.length" class="quota-grid">
      <article v-for="row in rows" :key="row.key" class="quota-item">
        <div class="quota-head">
          <span class="quota-dot" :class="row.tone" />
          <strong class="quota-label">{{ row.label }}</strong>
        </div>
        <div class="quota-track" :aria-label="`${row.label} 剩余 ${row.percent}%`">
          <div class="quota-fill" :class="row.tone" :style="{ width: `${row.percent}%` }" />
        </div>
        <span class="quota-caption">{{ row.caption }}</span>
      </article>
    </div>

    <div v-else class="quota-empty">
      <span>{{ quota?.message || '暂无可用额度信息' }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '../stores/app'
import type { QuotaSnapshot } from '../api'

const appStore = useAppStore()

const labels: Record<string, string> = {
  premium_interactions: '高级交互',
  chat: '聊天额度',
  completions: '补全额度',
}

const quota = computed(() => appStore.quota)

function snapshotPercent(snapshot: QuotaSnapshot): number {
  if (snapshot.unlimited) return 100
  if (typeof snapshot.percent_remaining === 'number') return clamp(snapshot.percent_remaining)
  const remaining = snapshot.remaining ?? snapshot.quota_remaining ?? 0
  const entitlement = snapshot.entitlement ?? 0
  if (!entitlement) return 0
  return clamp((remaining / entitlement) * 100)
}

function clamp(value: number): number {
  return Math.max(0, Math.min(100, Math.round(value)))
}

function toneFor(percent: number): 'quota-good' | 'quota-warn' | 'quota-danger' {
  if (percent > 60) return 'quota-good'
  if (percent > 25) return 'quota-warn'
  return 'quota-danger'
}

const rows = computed(() => {
  const snapshots = appStore.quota?.snapshots ?? {}
  return Object.keys(labels)
    .map((key) => {
      const snapshot = snapshots[key]
      if (!snapshot) return null
      const percent = snapshotPercent(snapshot)
      const remaining = snapshot.remaining ?? snapshot.quota_remaining ?? 0
      const entitlement = snapshot.entitlement ?? 0
      const caption = snapshot.unlimited ? '无限额度' : `${remaining}/${entitlement || '未知'} · ${percent}% 剩余`
      return {
        key,
        label: labels[key],
        percent,
        caption,
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
