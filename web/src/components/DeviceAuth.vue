<template>
  <Teleport to="body">
    <div v-if="modalShow" class="overlay" @click.self="close">
      <div class="device-dialog">
        <button class="dialog-close" @click="close">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
        </button>

        <div v-if="!flow && !loading" class="dialog-body">
          <div class="dialog-icon">
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
              <path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.4 5.4 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65-.17.6-.22 1.23-.15 1.85v4" />
              <path d="M9 18c-4.51 2-5-2-7-2" />
            </svg>
          </div>
          <h3 class="dialog-title">{{ t('deviceAuth.title') }}</h3>
          <p class="dialog-desc">{{ t('deviceAuth.requestingCode') }}</p>
        </div>

        <div v-else-if="loading && !flow" class="dialog-body">
          <div class="dialog-spinner" />
          <p class="dialog-desc">{{ t('deviceAuth.fetchingInfo') }}</p>
        </div>

        <div v-else-if="flow" class="dialog-body">
          <div class="dialog-header">
            <h3 class="dialog-title">{{ t('deviceAuth.authTitle') }}</h3>
            <p class="dialog-desc">{{ t('deviceAuth.authDesc') }}</p>
          </div>

          <div class="code-block">
            <span class="code-label">{{ t('deviceAuth.codeLabel') }}</span>
            <strong class="code-value">{{ flow.user_code }}</strong>
            <button class="code-copy" @click="copyCode">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
              </svg>
              {{ t('deviceAuth.copy') }}
            </button>
          </div>

          <div class="timer-track">
            <div class="timer-fill" :style="{ width: `${progress}%` }" />
          </div>

          <div class="dialog-footer">
            <span class="dialog-status">{{ statusText }}</span>
            <button class="dialog-btn dialog-btn--ghost" @click="close">{{ t('deviceAuth.cancel') }}</button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { deviceApi, type DeviceFlow } from '../api'
import { useI18n } from '../i18n'

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  (event: 'update:show', value: boolean): void
  (event: 'authorized'): void
}>()

const message = useMessage()
const { t } = useI18n()
const flow = ref<DeviceFlow | null>(null)
const loading = ref(false)
const polling = ref(false)
const statusText = ref(t('deviceAuth.waitingAuth'))
const startedAt = ref(0)
let pollTimer: number | undefined
let pollIntervalSeconds = 5
let flowRunId = 0

const modalShow = computed({
  get: () => props.show,
  set: (value: boolean) => {
    if (!value) clearPoll()
    emit('update:show', value)
  },
})

const progress = computed(() => {
  if (!flow.value || !startedAt.value) return 0
  const elapsed = Math.floor((Date.now() - startedAt.value) / 1000)
  return Math.min(100, Math.round((elapsed / flow.value.expires_in) * 100))
})

async function startFlow() {
  const runId = ++flowRunId
  loading.value = true
  statusText.value = t('deviceAuth.startDevice')
  clearPoll()
  try {
    const nextFlow = await deviceApi.start()
    if (!isCurrentRun(runId)) return
    flow.value = nextFlow
    try {
      await navigator.clipboard.writeText(flow.value.user_code)
      // Auto-copy succeeded - no toast needed to avoid being annoying
    } catch {
      message.info(t('deviceAuth.copyFail'))
    }
    pollIntervalSeconds = Math.max(1, flow.value.interval || 5)
    startedAt.value = Date.now()
    statusText.value = t('deviceAuth.waitConfirm')
    openBrowserTab()
    schedulePoll(runId)
  } catch (err) {
    if (!isCurrentRun(runId)) return
    message.error(err instanceof Error ? err.message : t('deviceAuth.startFail'))
    emit('update:show', false)
  } finally {
    if (isCurrentRun(runId)) loading.value = false
  }
}

function schedulePoll(runId: number) {
  clearPoll()
  pollTimer = window.setTimeout(() => {
    poll(runId).catch((err) => {
      if (!isCurrentRun(runId)) return
      statusText.value = err instanceof Error ? err.message : t('deviceAuth.pollFail')
      schedulePoll(runId)
    })
  }, pollIntervalSeconds * 1000)
}

async function poll(runId: number) {
  if (!flow.value || polling.value) return
  polling.value = true
  try {
    let result
    try {
      result = await deviceApi.poll()
    } catch (err) {
      if (!isCurrentRun(runId)) return
      // 非 2xx 响应：终端 OAuth 错误（expired_token / access_denied 等），关闭弹窗
      clearPoll()
      message.error(t('deviceAuth.authExpired'))
      close()
      return
    }
    if (!isCurrentRun(runId)) return
    if (result.status === 'authorized') {
      message.success(t('deviceAuth.authSuccess'))
      clearPoll()
      emit('authorized')
      emit('update:show', false)
      return
    }
    if (result.status === 'slow_down') pollIntervalSeconds += 5
    if (result.status === 'expired' || result.status === 'expired_token' || result.status === 'access_denied') {
      message.error(t('deviceAuth.authExpired'))
      close()
      return
    }
    statusText.value = statusLabel(result.status)
    schedulePoll(runId)
  } finally {
    if (isCurrentRun(runId)) polling.value = false
  }
}

function statusLabel(status: string): string {
  const labels: Record<string, string> = {
    pending: t('deviceAuth.statusPending'),
    authorization_pending: t('deviceAuth.statusAuthorizationPending'),
    slow_down: t('deviceAuth.statusSlowDown'),
  }
  return labels[status] ?? t('deviceAuth.statusDefault', { status })
}

async function copyCode() {
  if (!flow.value) return
  try {
    await navigator.clipboard.writeText(flow.value.user_code)
    message.success(t('deviceAuth.copied'))
  } catch {
    message.warning(t('deviceAuth.copyFail'))
  }
}

function openBrowserTab() {
  if (!flow.value?.verification_uri) return
  const tab = window.open(flow.value.verification_uri, '_blank', 'noopener,noreferrer')
  if (!tab || tab.closed) {
    message.warning(t('deviceAuth.popupBlocked'))
  }
}

function clearPoll() {
  if (pollTimer) window.clearTimeout(pollTimer)
  pollTimer = undefined
}

function isCurrentRun(runId: number): boolean {
  return props.show && flowRunId === runId
}

function close() {
  flowRunId++
  clearPoll()
  flow.value = null
  polling.value = false
  emit('update:show', false)
}

watch(
  () => props.show,
  (show) => {
    if (show) startFlow()
    else close()
  },
)

onUnmounted(clearPoll)
</script>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 5000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  animation: fadeIn 200ms ease;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.device-dialog {
  position: relative;
  width: 380px;
  background: var(--cp-color-card);
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-lg);
  box-shadow: var(--cp-shadow-float);
  animation: scaleIn 250ms cubic-bezier(0.34, 1.56, 0.64, 1);
  overflow: hidden;
}

@keyframes scaleIn {
  from { opacity: 0; transform: scale(0.92); }
  to { opacity: 1; transform: scale(1); }
}

.dialog-close {
  position: absolute;
  top: 12px;
  right: 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--cp-color-text-muted);
  cursor: pointer;
  transition: all var(--cp-transition-fast);
  z-index: 1;
}

.dialog-close:hover {
  background: var(--cp-color-border);
  color: var(--cp-color-text);
}

/* Body */
.dialog-body {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--cp-space-4);
  padding: var(--cp-space-8) var(--cp-space-6) var(--cp-space-6);
  text-align: center;
}

.dialog-header {
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-1);
}

.dialog-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  border-radius: 50%;
  background: var(--cp-color-primary-soft);
  color: var(--cp-color-primary);
  margin-bottom: var(--cp-space-1);
}

.dialog-title {
  margin: 0;
  font-size: var(--cp-font-size-lg);
  font-weight: 700;
  color: var(--cp-color-text);
}

.dialog-desc {
  margin: 0;
  font-size: var(--cp-font-size-sm);
  color: var(--cp-color-text-muted);
  line-height: 1.5;
}

/* Code block */
.code-block {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--cp-space-2);
  width: 100%;
  padding: var(--cp-space-5) var(--cp-space-4);
  border: 1px dashed var(--cp-color-primary);
  border-radius: var(--cp-radius-md);
  background: var(--cp-color-primary-soft);
}

.code-label {
  font-size: var(--cp-font-size-xs);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--cp-color-text-muted);
}

.code-value {
  font-size: 36px;
  font-weight: 800;
  letter-spacing: 0.16em;
  color: var(--cp-color-text);
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', 'Consolas', monospace;
}

.code-copy {
  display: inline-flex;
  align-items: center;
  gap: var(--cp-space-1);
  padding: var(--cp-space-1) var(--cp-space-3);
  font-size: var(--cp-font-size-xs);
  font-weight: 600;
  border: 1px solid var(--cp-color-primary);
  border-radius: var(--cp-radius-sm);
  background: transparent;
  color: var(--cp-color-primary);
  cursor: pointer;
  transition: all var(--cp-transition-fast);
  outline: none;
}

.code-copy:hover {
  background: var(--cp-color-primary);
  color: #fff;
}

/* Timer */
.timer-track {
  width: 100%;
  height: 4px;
  border-radius: 2px;
  background: var(--cp-color-border);
  overflow: hidden;
}

.timer-fill {
  height: 100%;
  border-radius: inherit;
  background: var(--cp-color-primary);
  transition: width 1s linear;
}

/* Footer */
.dialog-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  margin-top: var(--cp-space-1);
}

.dialog-status {
  font-size: var(--cp-font-size-xs);
  color: var(--cp-color-text-muted);
}

.dialog-btn {
  padding: var(--cp-space-1) var(--cp-space-4);
  font-size: var(--cp-font-size-sm);
  border-radius: var(--cp-radius-sm);
  cursor: pointer;
  border: 1px solid var(--cp-color-border);
  outline: none;
  transition: all var(--cp-transition-fast);
  background: transparent;
  color: var(--cp-color-text-secondary);
}

.dialog-btn:hover {
  border-color: var(--cp-color-text-muted);
}

/* Spinner */
.dialog-spinner {
  width: 28px;
  height: 28px;
  border: 3px solid var(--cp-color-border);
  border-top-color: var(--cp-color-primary);
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
