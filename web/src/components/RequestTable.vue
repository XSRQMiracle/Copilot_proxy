<template>
  <div class="request-panel">
    <div v-if="records.length" class="request-table-wrap">
      <table class="request-table">
        <thead>
          <tr>
            <th>时间</th>
            <th>协议</th>
            <th>模型</th>
            <th>状态</th>
            <th>Token</th>
            <th>耗时</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="record in records" :key="record.id">
            <tr
              class="request-row"
              :class="record.success ? '' : 'request-row--failed'"
              @click="toggle(record.id)"
            >
              <td>{{ formatTime(record.time) }}</td>
              <td>{{ protocolLabel(record.protocol) }}</td>
              <td class="request-model">{{ record.model || 'unknown' }}</td>
              <td>
                <span class="req-status" :class="record.success ? 'req-ok' : 'req-err'">
                  {{ record.success ? '成功' : '失败' }}
                </span>
              </td>
              <td>{{ formatNumber(record.total_tokens) }}</td>
              <td>{{ record.duration_ms }}ms</td>
            </tr>
            <tr v-if="expandedId === record.id" class="request-detail-row">
              <td colspan="6">
                <div class="request-detail">
                  <span><strong>方法：</strong>{{ record.method }}</span>
                  <span><strong>路径：</strong>{{ record.path }}</span>
                  <span><strong>输入：</strong>{{ formatNumber(record.prompt_tokens) }}</span>
                  <span><strong>输出：</strong>{{ formatNumber(record.completion_tokens) }}</span>
                  <span v-if="record.error" class="request-error"><strong>错误：</strong>{{ record.error }}</span>
                </div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <div v-else class="request-empty">
      <span>暂无请求记录 — 发起 OpenAI、Anthropic 或 Gemini 兼容请求后自动记录</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAppStore } from '../stores/app'

const appStore = useAppStore()
const expandedId = ref<number | null>(null)

const records = computed(() => appStore.stats?.recent ?? [])

function toggle(id: number) {
  expandedId.value = expandedId.value === id ? null : id
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat('zh-CN').format(value ?? 0)
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(date)
}

function protocolLabel(protocol: string): string {
  const normalized = protocol.toLowerCase()
  if (normalized.includes('anthropic')) return 'Anthropic'
  if (normalized.includes('gemini')) return 'Gemini'
  return 'OpenAI'
}
</script>

<style scoped>
.request-panel {
  width: 100%;
}

.request-table-wrap {
  overflow-x: auto;
}

.request-table {
  width: 100%;
  border-collapse: collapse;
  min-width: 640px;
}

.request-table th,
.request-table td {
  padding: var(--cp-space-2) var(--cp-space-3);
  text-align: left;
  border-bottom: 1px solid var(--cp-color-border);
  font-size: var(--cp-font-size-sm);
  color: var(--cp-color-text);
}

.request-table th {
  font-size: var(--cp-font-size-xs);
  font-weight: 600;
  color: var(--cp-color-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.request-row {
  cursor: pointer;
  transition: background var(--cp-transition-fast);
}

.request-row:hover {
  background: var(--cp-color-primary-soft);
}

.request-row--failed {
  background: var(--cp-color-error-soft);
}

.request-model {
  max-width: 240px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  font-size: var(--cp-font-size-xs);
}

.req-status {
  display: inline-flex;
  padding: 2px 10px;
  font-size: var(--cp-font-size-xs);
  font-weight: 600;
  border-radius: 999px;
}

.req-ok {
  background: var(--cp-color-success-soft);
  color: var(--cp-color-success);
}

.req-err {
  background: var(--cp-color-error-soft);
  color: var(--cp-color-error);
}

.request-detail-row td {
  background: var(--cp-color-primary-soft);
  padding: var(--cp-space-3);
}

.request-detail {
  display: flex;
  flex-wrap: wrap;
  gap: var(--cp-space-2) var(--cp-space-4);
  font-size: var(--cp-font-size-xs);
  color: var(--cp-color-text-secondary);
}

.request-error {
  color: var(--cp-color-error);
  width: 100%;
}

.request-empty {
  padding: var(--cp-space-4) 0;
  text-align: center;
  font-size: var(--cp-font-size-sm);
  color: var(--cp-color-text-muted);
}
</style>
