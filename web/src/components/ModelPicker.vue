<template>
  <div class="model-panel">
    <div class="model-toolbar">
      <button class="model-tb-btn" :disabled="loading" @click="loadData">
        {{ loading ? t('modelPicker.refreshing') : t('modelPicker.refresh') }}
      </button>
    </div>

    <n-spin :show="loading">
      <div class="model-sections">
        <section class="model-section">
          <div class="model-section__head">
            <span>{{ t('modelPicker.availableModels') }}</span>
            <span class="model-count">{{ availableModels.length }}</span>
          </div>
          <div v-if="availableModels.length" class="model-scroll">
            <article v-for="m in availableModels" :key="m.id" class="model-item" @click="addModel(m.id)">
              <div class="model-item__info">
                <strong>{{ m.name || m.id }}</strong>
                <span>{{ m.vendor || 'Copilot' }}</span>
              </div>
              <span class="model-item__add">+</span>
            </article>
          </div>
          <div v-else class="model-empty">{{ t('modelPicker.noModelsToAdd') }}</div>
        </section>

        <section class="model-section">
          <div class="model-section__head">
            <span>{{ t('modelPicker.fallbackList') }}</span>
            <span class="model-count">{{ fallbackList.length }}</span>
          </div>

          <div class="model-add-row">
            <input
              v-model="manualModel"
              class="model-input"
              :placeholder="t('modelPicker.manualPlaceholder')"
              @keyup.enter="addManualModel"
            />
            <button class="model-add-btn" @click="addManualModel">{{ t('modelPicker.add') }}</button>
          </div>

          <div v-if="fallbackList.length" class="model-scroll">
            <article v-for="(model, i) in fallbackList" :key="i" class="model-fallback">
              <span class="model-idx">{{ i + 1 }}</span>
              <strong class="model-fb-name">{{ model }}</strong>
              <div class="model-fb-actions">
                <button :disabled="i === 0" @click="moveModel(i, -1)">↑</button>
                <button :disabled="i === fallbackList.length - 1" @click="moveModel(i, 1)">↓</button>
                <button class="fb-del" @click="removeModel(i)">×</button>
              </div>
            </article>
          </div>
          <div v-else class="model-empty">{{ t('modelPicker.emptyFallback') }}</div>
        </section>
      </div>

      <div class="model-actions">
        <button class="field-btn field-btn--ghost" @click="resetFallback">{{ t('modelPicker.reset') }}</button>
        <button class="field-btn field-btn--primary" :disabled="saving" @click="saveFallback">
          {{ saving ? t('modelPicker.saving') : t('modelPicker.confirmSave') }}
        </button>
      </div>
    </n-spin>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { configApi, fallbackApi, modelsApi, statusApi, type ModelItem } from '../api'
import { useI18n } from '../i18n'
import { useAppStore } from '../stores/app'

const appStore = useAppStore()
const message = useMessage()
const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const fallbackList = ref<string[]>([])
const manualModel = ref('')

const availableModels = computed(() => appStore.models
  .filter((model) => model.id && model.available !== false)
  .filter((model) => !fallbackList.value.includes(model.id))
  .sort((a, b) => a.id.localeCompare(b.id)))

function resetFallback() {
  fallbackList.value = [...(appStore.config?.fallback.preferred_prefixes ?? [])]
}

function addModel(id: string) {
  const trimmed = id.trim()
  if (!trimmed || fallbackList.value.includes(trimmed)) return
  fallbackList.value.push(trimmed)
}

function addManualModel() {
  addModel(manualModel.value)
  manualModel.value = ''
}

function removeModel(index: number) {
  fallbackList.value.splice(index, 1)
}

function moveModel(index: number, direction: -1 | 1) {
  const target = index + direction
  if (target < 0 || target >= fallbackList.value.length) return
  const [item] = fallbackList.value.splice(index, 1)
  fallbackList.value.splice(target, 0, item)
}

async function ensureConfig() {
  if (appStore.config) return appStore.config
  const cfg = await configApi.get()
  appStore.setConfig(cfg)
  return cfg
}

async function loadData() {
  loading.value = true
  try {
    const [cfg, modelResponse] = await Promise.all([
      ensureConfig(),
      modelsApi.list().catch(() => ({ data: [] as ModelItem[] })),
    ])
    appStore.models = modelResponse.data ?? []
    fallbackList.value = [...cfg.fallback.preferred_prefixes]
  } catch (err) {
    message.error(err instanceof Error ? err.message : t('modelPicker.loadError'))
  } finally {
    loading.value = false
  }
}

async function saveFallback() {
  saving.value = true
  try {
    const unique = [...new Set(fallbackList.value.map((item) => item.trim()).filter(Boolean))]
    const result = await fallbackApi.update(unique)
    fallbackList.value = result.preferred_prefixes
    if (appStore.config) appStore.config.fallback.preferred_prefixes = result.preferred_prefixes
    appStore.status = await statusApi.get().catch(() => appStore.status)
    message.success(t('modelPicker.saveSuccess'))
  } catch (err) {
    message.error(err instanceof Error ? err.message : t('modelPicker.saveFail'))
  } finally {
    saving.value = false
  }
}

onMounted(loadData)
</script>

<style scoped>
.model-panel {
  width: 100%;
}

.model-toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: var(--cp-space-3);
}

.model-tb-btn {
  padding: var(--cp-space-1) var(--cp-space-3);
  font-size: var(--cp-font-size-xs);
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-sm);
  background: transparent;
  color: var(--cp-color-text-secondary);
  cursor: pointer;
  transition: all var(--cp-transition-fast);
  outline: none;
}

.model-tb-btn:hover {
  border-color: var(--cp-color-primary);
  color: var(--cp-color-primary);
}

.model-tb-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.model-sections {
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-4);
}

.model-section {
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-2);
}

.model-section__head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: var(--cp-font-size-xs);
  font-weight: 600;
  color: var(--cp-color-text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.model-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 4px;
  font-size: 10px;
  border-radius: 9px;
  background: var(--cp-color-border);
  color: var(--cp-color-text-muted);
}

.model-scroll {
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-1);
  max-height: 200px;
  overflow-y: auto;
}

.model-item,
.model-fallback {
  display: flex;
  align-items: center;
  gap: var(--cp-space-2);
  padding: var(--cp-space-2) var(--cp-space-3);
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-sm);
  cursor: pointer;
  transition: all var(--cp-transition-fast);
}

.model-item:hover {
  border-color: var(--cp-color-primary);
  background: var(--cp-color-primary-soft);
}

.model-item__info {
  flex: 1;
  min-width: 0;
}

.model-item__info strong {
  display: block;
  font-size: var(--cp-font-size-sm);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-item__info span {
  font-size: var(--cp-font-size-xs);
  color: var(--cp-color-text-muted);
}

.model-item__add {
  color: var(--cp-color-primary);
  font-size: 16px;
  font-weight: 600;
  line-height: 1;
}

.model-add-row {
  display: flex;
  gap: var(--cp-space-2);
}

.model-input {
  flex: 1;
  padding: var(--cp-space-1) var(--cp-space-3);
  font-size: var(--cp-font-size-sm);
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-sm);
  background: transparent;
  color: var(--cp-color-text);
  outline: none;
  transition: border-color var(--cp-transition-fast);
}

.model-input:focus {
  border-color: var(--cp-color-primary);
}

.model-add-btn {
  padding: var(--cp-space-1) var(--cp-space-3);
  font-size: var(--cp-font-size-xs);
  border: 1px solid var(--cp-color-primary);
  border-radius: var(--cp-radius-sm);
  background: var(--cp-color-primary);
  color: #fff;
  cursor: pointer;
  transition: all var(--cp-transition-fast);
  outline: none;
}

.model-add-btn:hover {
  opacity: 0.9;
}

.model-fallback {
  cursor: default;
}

.model-idx {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  font-size: 10px;
  font-weight: 700;
  border-radius: 50%;
  background: var(--cp-color-border);
  color: var(--cp-color-text-muted);
  flex-shrink: 0;
}

.model-fb-name {
  flex: 1;
  font-size: var(--cp-font-size-sm);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-fb-actions {
  display: flex;
  gap: 2px;
}

.model-fb-actions button {
  width: 22px;
  height: 22px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--cp-color-border);
  border-radius: 4px;
  background: transparent;
  color: var(--cp-color-text-muted);
  font-size: 12px;
  cursor: pointer;
  transition: all var(--cp-transition-fast);
  outline: none;
}

.model-fb-actions button:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.model-fb-actions button:hover:not(:disabled) {
  border-color: var(--cp-color-primary);
  color: var(--cp-color-primary);
}

.fb-del:hover {
  border-color: var(--cp-color-error) !important;
  color: var(--cp-color-error) !important;
}

.model-empty {
  text-align: center;
  padding: var(--cp-space-3);
  font-size: var(--cp-font-size-xs);
  color: var(--cp-color-text-muted);
}

.model-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--cp-space-2);
  margin-top: var(--cp-space-3);
}

.field-btn {
  padding: var(--cp-space-1) var(--cp-space-4);
  font-size: var(--cp-font-size-sm);
  border-radius: var(--cp-radius-sm);
  cursor: pointer;
  border: 1px solid var(--cp-color-border);
  outline: none;
  transition: all var(--cp-transition-fast);
}

.field-btn--ghost {
  background: transparent;
  color: var(--cp-color-text-secondary);
}

.field-btn--ghost:hover {
  background: var(--cp-color-border);
}

.field-btn--primary {
  background: var(--cp-color-primary);
  color: #fff;
  border-color: var(--cp-color-primary);
}

.field-btn--primary:hover {
  opacity: 0.9;
}

.field-btn--primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
