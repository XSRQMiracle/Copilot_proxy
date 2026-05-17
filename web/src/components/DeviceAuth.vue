<template>
  <n-modal v-model:show="modalShow" preset="card" class="device-modal" :bordered="false" title="GitHub 设备授权">
    <n-spin :show="loading">
      <n-space v-if="flow" vertical :size="20">
        <n-alert type="info" :show-icon="true">
          请在 GitHub 授权页面输入下方验证码。授权完成后会自动返回控制台。
        </n-alert>

        <section class="device-code" @click="copyCode">
          <n-text depth="3">验证码</n-text>
          <strong>{{ flow.user_code }}</strong>
          <n-button secondary size="small" @click.stop="copyCode">复制 Code</n-button>
        </section>

        <n-space vertical :size="8">
          <n-text depth="3">授权地址</n-text>
          <a
            v-if="safeVerificationUri"
            class="device-link"
            :href="safeVerificationUri"
            target="_blank"
            rel="noopener noreferrer"
            @click="openBrowserTab"
          >
            {{ safeVerificationUri }}
          </a>
          <n-button
            v-if="safeVerificationUri"
            secondary
            size="small"
            class="device-open-btn"
            @click="openBrowserTab"
          >
            重新打开 GitHub
          </n-button>
          <n-alert v-else type="error" :show-icon="true">授权地址校验失败，请重新开始授权。</n-alert>
        </n-space>

        <n-progress type="line" :percentage="progress" :show-indicator="false" />

        <n-space justify="space-between" align="center">
          <n-text depth="3">{{ statusText }}</n-text>
          <n-button tertiary @click="close">取消</n-button>
        </n-space>
      </n-space>

      <n-empty v-else description="正在创建授权流程" />
    </n-spin>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { deviceApi, type DeviceFlow } from '../api'

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  (event: 'update:show', value: boolean): void
  (event: 'authorized'): void
}>()

const message = useMessage()
const flow = ref<DeviceFlow | null>(null)
const loading = ref(false)
const polling = ref(false)
const statusText = ref('等待 GitHub 授权确认…')
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

const safeVerificationUri = computed(() => {
  if (!flow.value?.verification_uri) return ''
  try {
    const url = new URL(flow.value.verification_uri)
    const allowedHosts = new Set(['github.com', 'www.github.com'])
    if (url.protocol !== 'https:' || !allowedHosts.has(url.hostname)) return ''
    if (url.pathname !== '/login/device') return ''
    return url.toString()
  } catch {
    return ''
  }
})

async function startFlow() {
  const runId = ++flowRunId
  loading.value = true
  statusText.value = '正在向 GitHub 申请验证码…'
  clearPoll()
  try {
    const nextFlow = await deviceApi.start()
    if (!isCurrentRun(runId)) return
    flow.value = nextFlow
    pollIntervalSeconds = Math.max(1, flow.value.interval || 5)
    startedAt.value = Date.now()
    statusText.value = '等待你在 GitHub 页面确认授权…'
    openBrowserTab()
    schedulePoll(runId)
  } catch (err) {
    if (!isCurrentRun(runId)) return
    message.error(err instanceof Error ? err.message : '设备授权启动失败')
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
      statusText.value = err instanceof Error ? err.message : '轮询授权状态失败'
      schedulePoll(runId)
    })
  }, pollIntervalSeconds * 1000)
}

async function poll(runId: number) {
  if (!flow.value || polling.value) return
  polling.value = true
  try {
    const result = await deviceApi.poll()
    if (!isCurrentRun(runId)) return
    if (result.status === 'authorized') {
      message.success('GitHub 授权成功')
      clearPoll()
      emit('authorized')
      emit('update:show', false)
      return
    }
    if (result.status === 'slow_down') pollIntervalSeconds += 5
    if (result.status === 'expired' || result.status === 'expired_token' || result.status === 'access_denied') {
      message.error('授权已过期或被拒绝，请重新开始')
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
    pending: '等待 GitHub 授权确认…',
    authorization_pending: '还没有确认授权，继续等待中…',
    slow_down: 'GitHub 要求降低轮询频率，已自动放慢…',
  }
  return labels[status] ?? `当前状态：${status}`
}

async function copyCode() {
  if (!flow.value) return
  try {
    await navigator.clipboard.writeText(flow.value.user_code)
    message.success('验证码已复制')
  } catch {
    message.warning('浏览器不允许自动复制，请手动复制验证码')
  }
}

function openBrowserTab() {
  if (!flow.value?.verification_uri) return
  const tab = window.open(flow.value.verification_uri, '_blank', 'noopener,noreferrer')
  if (!tab || tab.closed) {
    message.warning('浏览器弹窗被拦截，请手动点击链接打开')
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
.device-modal {
  width: min(92vw, 560px);
  border-radius: var(--cp-radius-lg);
  box-shadow: var(--cp-shadow-float);
}

.device-code {
  display: grid;
  justify-items: center;
  gap: var(--cp-space-3);
  padding: var(--cp-space-8);
  border: 1px dashed var(--cp-color-primary);
  border-radius: var(--cp-radius-lg);
  background: var(--cp-color-primary-soft);
  cursor: copy;
}

.device-code strong {
  font-size: clamp(var(--cp-font-size-2xl), 8vw, 56px);
  letter-spacing: 0.12em;
}

.device-link {
  color: var(--cp-color-primary);
  word-break: break-all;
  text-decoration: none;
}

.device-open-btn {
  align-self: flex-start;
}
</style>
