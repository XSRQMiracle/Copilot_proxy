<template>
  <div class="chat-test-card" data-testid="chat-test-panel">
    <div class="chat-test-header">
      <h2>{{ t('chatTest.panelTitle') }}</h2>
    </div>
    <div class="chat-test-body">
      <!-- 左侧：模型选择 -->
      <div class="ctp-sidebar">
        <div class="ctp-sidebar-header">
          <h2>{{ t('chatTest.modelSelector') }}</h2>
        </div>
        <div class="ctp-sidebar-body">
          <div v-if="loading" class="ctp-muted">{{ t('chatTest.loadingModels') }}</div>
          <div v-else-if="modelError" class="ctp-error">{{ modelError }}</div>
          <template v-else>
            <input v-model="modelSearch" class="ctp-search" :placeholder="t('chatTest.searchModels')" />
            <div class="ctp-model-list">
              <div
                v-for="m in filteredModels"
                :key="m.id"
                class="ctp-model-item"
                :class="{ active: selectedModel === m.id }"
                @click="selectModel(m.id)"
              >
                <span class="ctp-model-name">{{ displayModelName(m) }}</span>
                <span v-if="m.vendor" class="ctp-model-vendor">{{ m.vendor }}</span>
              </div>
              <div v-if="filteredModels.length === 0" class="ctp-muted">{{ t('chatTest.noModelsMatch') }}</div>
            </div>
          </template>
        </div>
      </div>

      <!-- 右侧：对话 -->
      <div class="ctp-main">
        <div class="ctp-chat-header">
          <h2>{{ t('chatTest.chatTitle') }}</h2>
          <span v-if="currentModelInfo" class="ctp-current-model">{{ displayModelName(currentModelInfo) }}</span>
        </div>
        <div class="ctp-chat-body" ref="chatBodyRef">
          <div class="ctp-messages">
            <div v-for="(msg, i) in messages" :key="i" class="ctp-msg-row" :class="msg.role">
              <div class="ctp-msg-avatar">{{ msg.role === 'user' ? 'U' : 'A' }}</div>
              <div class="ctp-msg-content">
                <div class="ctp-msg-text">{{ msg.content }}</div>
              </div>
            </div>
            <div v-if="streamingText" class="ctp-msg-row assistant">
              <div class="ctp-msg-avatar">A</div>
              <div class="ctp-msg-content">
                <div class="ctp-msg-text ctp-streaming">{{ streamingText }}<span class="ctp-cursor">▊</span></div>
              </div>
            </div>
          </div>
        </div>

        <div class="ctp-composer">
          <div class="ctp-composer-row">
            <textarea
              v-model="inputText"
              class="ctp-input"
              :placeholder="t('chatTest.inputPlaceholder')"
              :disabled="!selectedModel || sending"
              @keydown.enter.exact.prevent="sendMessage"
            />
            <button class="ctp-send-btn" :disabled="!canSend" @click="sendMessage">
              {{ sending ? '...' : t('chatTest.send') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { chatApi, type ChatMessage, type ModelInfo } from '../api'
import { createChatStream, readStream } from '../utils/stream'
import { useI18n } from '../i18n'

const { t } = useI18n()

const models = ref<ModelInfo[]>([])
const loading = ref(false)
const modelError = ref('')
const selectedModel = ref('')
const modelSearch = ref('')

const filteredModels = computed(() => {
  const q = modelSearch.value.toLowerCase().trim()
  if (!q) return models.value
  return models.value.filter(m => {
    const id = (m.id ?? '').toLowerCase()
    const name = (m.name ?? '').toLowerCase()
    const vendor = (m.vendor ?? '').toLowerCase()
    return id.includes(q) || name.includes(q) || vendor.includes(q)
  })
})

const currentModelInfo = computed(() => models.value.find(m => m.id === selectedModel.value) ?? null)

const messages = ref<ChatMessage[]>([])
const inputText = ref('')
const sending = ref(false)
const streamingText = ref('')
const chatBodyRef = ref<HTMLElement | null>(null)

function scrollToBottom() {
  nextTick(() => {
    if (chatBodyRef.value) {
      chatBodyRef.value.scrollTop = chatBodyRef.value.scrollHeight
    }
  })
}

const canSend = computed(() => selectedModel.value && inputText.value.trim() && !sending.value)

function displayModelName(m: ModelInfo): string {
  return m.name || m.id
}

function selectModel(id: string) {
  selectedModel.value = id
}

async function sendMessage() {
  if (!canSend.value) return
  const text = inputText.value.trim()
  inputText.value = ''

  messages.value = [{ role: 'user', content: text }]
  scrollToBottom()

  sending.value = true
  streamingText.value = ''

  try {
    const reader = await createChatStream(selectedModel.value, [{ role: 'user', content: text }])
    await readStream(reader, {
      onDelta: (text) => { 
        streamingText.value += text
        scrollToBottom()
      },
      onDone: () => {
        if (streamingText.value.trim().length > 0) {
          messages.value.push({ role: 'assistant', content: streamingText.value })
        }
        streamingText.value = ''
        scrollToBottom()
      },
      onError: (err) => {
        messages.value.push({ role: 'assistant', content: `Error: ${err.message}` })
        streamingText.value = ''
        scrollToBottom()
      },
    })
  } catch (err) {
    messages.value.push({ role: 'assistant', content: `Error: ${err instanceof Error ? err.message : String(err)}` })
    scrollToBottom()
  } finally {
    sending.value = false
  }
}

onMounted(async () => {
  loading.value = true
  try {
    const resp = await chatApi.models()
    const raw = resp.data ?? []
    // 归一化：确保每个模型都有非空 id，并过滤不可用模型
    models.value = raw
      .map(m => ({ ...m, id: m.id || m.name || `model-${Math.random().toString(36).slice(2, 8)}` }))
      .filter(m => m.policy?.state !== 'disabled' && m.model_picker_enabled !== false)
    if (models.value.length > 0) {
      selectedModel.value = models.value[0].id
    }
  } catch (err) {
    modelError.value = err instanceof Error ? err.message : String(err)
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
/* ── 外层 card ── */
.chat-test-card {
  background: var(--cp-color-surface);
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-lg);
  backdrop-filter: blur(3px) saturate(180%) brightness(1.06);
  -webkit-backdrop-filter: blur(3px) saturate(180%) brightness(1.06);
  overflow: hidden;
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.chat-test-header {
  padding: var(--cp-space-4) var(--cp-space-5) 0;
  flex-shrink: 0;
}

.chat-test-header h2 {
  margin: 0;
  font-size: var(--cp-font-size-sm);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--cp-color-text-muted);
}

.chat-test-body {
  flex-shrink: 0;
  height: 65vh;
  display: grid;
  grid-template-columns: minmax(200px, 1fr) 3fr;
  grid-template-rows: minmax(0, 1fr);
  overflow: hidden;
}

/* ── 左侧：模型选择 ── */
.ctp-sidebar {
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--cp-color-border);
  min-height: 0;
  height: 100%;
  overflow: hidden;
}

.ctp-sidebar-header {
  padding: var(--cp-space-4) var(--cp-space-5) 0;
  flex-shrink: 0;
}

.ctp-sidebar-header h2 {
  margin: 0;
  font-size: var(--cp-font-size-sm);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--cp-color-text-muted);
}

.ctp-sidebar-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: var(--cp-space-4) var(--cp-space-5);
}

/* ── 右侧：对话 ── */
.ctp-main {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  overflow: hidden;
}

.ctp-chat-header {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  padding: var(--cp-space-4) var(--cp-space-5) 0;
  flex-shrink: 0;
}

.ctp-chat-header h2 {
  margin: 0;
  font-size: var(--cp-font-size-sm);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--cp-color-text-muted);
}

.ctp-current-model {
  justify-self: center;
  font-size: var(--cp-font-size-xs);
  color: var(--cp-color-primary);
  background: var(--cp-color-primary-soft);
  padding: 1px 8px;
  border-radius: var(--cp-radius-sm);
  font-weight: 500;
  line-height: 1.6;
  max-width: 160px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* ── 公共 ── */
.ctp-muted {
  color: var(--cp-color-text-secondary);
  padding: var(--cp-space-3);
}

.ctp-error {
  color: var(--cp-color-error);
  padding: var(--cp-space-3);
}

.ctp-search {
  width: 100%;
  padding: var(--cp-space-1) var(--cp-space-2);
  margin-bottom: var(--cp-space-2);
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-sm);
  background: var(--cp-color-surface);
  color: var(--cp-color-text);
  font-size: var(--cp-font-size-xs);
  box-sizing: border-box;
}

.ctp-search:focus {
  outline: none;
  border-color: var(--cp-color-primary);
}

/* ── 模型列表（左侧可滚动） ── */
.ctp-model-list {
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-1);
}

.ctp-model-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--cp-space-2) var(--cp-space-3);
  border-radius: var(--cp-radius-sm);
  cursor: pointer;
  transition: background var(--cp-transition-fast);
}

.ctp-model-item:hover {
  background: var(--cp-color-primary-soft);
}

.ctp-model-item.active {
  background: var(--cp-color-primary-soft);
  border: 1px solid var(--cp-color-primary);
}

.ctp-model-name {
  font-size: var(--cp-font-size-sm);
  font-weight: 500;
  color: var(--cp-color-text);
}

.ctp-model-vendor {
  font-size: var(--cp-font-size-xs);
  color: var(--cp-color-text-muted);
}

/* ── 消息列表（右侧可滚动） ── */
.ctp-chat-body {
  flex: 1;
  overflow-y: auto;
  padding: var(--cp-space-4) var(--cp-space-5);
  min-height: 0;
}

.ctp-messages {
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-4);
}

.ctp-msg-row {
  display: flex;
  gap: var(--cp-space-3);
}

.ctp-msg-row.user {
  flex-direction: row-reverse;
}

.ctp-msg-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--cp-font-size-xs);
  font-weight: 600;
  flex-shrink: 0;
  background: var(--cp-color-primary-soft);
  color: var(--cp-color-primary);
}

.ctp-msg-row.user .ctp-msg-avatar {
  background: var(--cp-color-primary);
  color: var(--cp-color-text-on-primary, #fff);
}

.ctp-msg-content {
  max-width: 80%;
}

.ctp-msg-text {
  padding: var(--cp-space-2) var(--cp-space-3);
  border-radius: var(--cp-radius-md);
  background: var(--cp-color-card);
  border: 1px solid var(--cp-color-border);
  white-space: pre-wrap;
  word-break: break-word;
  font-size: var(--cp-font-size-sm);
  line-height: 1.5;
}

.ctp-msg-row.user .ctp-msg-text {
  background: var(--cp-color-primary-soft);
}

.ctp-cursor {
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  50% { opacity: 0; }
}

/* ── 输入区 ── */
.ctp-composer {
  border-top: 1px solid var(--cp-color-border);
  padding: var(--cp-space-4) var(--cp-space-5);
  flex-shrink: 0;
}

.ctp-composer-row {
  display: flex;
  gap: var(--cp-space-3);
}

.ctp-input {
  flex: 1;
  resize: none;
  padding: var(--cp-space-2) var(--cp-space-3);
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-sm);
  background: #fff;
  color: #1a1a2e;
  font-size: var(--cp-font-size-sm);
  line-height: 1.5;
  min-height: 38px;
  max-height: 160px;
}

.ctp-input:focus {
  outline: none;
  border-color: var(--cp-color-primary);
}

.ctp-send-btn {
  height: 42px;
  line-height: 42px;
  padding: 0 var(--cp-space-4);
  border: 1px solid var(--cp-color-primary);
  border-radius: var(--cp-radius-sm);
  background: var(--cp-color-primary);
  color: var(--cp-color-text-on-primary, #fff);
  font-size: var(--cp-font-size-sm);
  font-weight: 500;
  cursor: pointer;
  align-self: center;
  transition: opacity var(--cp-transition-fast);
}

.ctp-send-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ctp-send-btn:hover:not(:disabled) {
  opacity: 0.85;
}

@media (max-width: 1024px) {
  .chat-test-body {
    grid-template-columns: 1fr;
  }
}
</style>
