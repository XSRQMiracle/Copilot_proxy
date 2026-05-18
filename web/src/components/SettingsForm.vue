<template>
  <div class="settings-panel">
    <n-spin :show="loading">
      <div v-if="form" class="settings-fields">
        <div class="field-row">
          <div class="field-col">
            <label>{{ t('settingsForm.listenAddress') }}</label>
            <n-input v-model:value="form.server.host" placeholder="0.0.0.0" size="small" />
          </div>
          <div class="field-col">
            <label>{{ t('settingsForm.listenPort') }}</label>
            <n-input-number v-model:value="form.server.port" :min="1" :max="65535" :show-button="false" size="small" class="field-number" />
          </div>
        </div>

        <div class="field-row">
          <div class="field-col">
            <label>{{ t('settingsForm.apiKey') }}</label>
            <n-input v-model:value="form.security.api_key" type="password" show-password-on="click" size="small" />
          </div>
          <div class="field-col">
            <label>API Base</label>
            <n-input v-model:value="form.copilot.api_base" placeholder="https://api.githubcopilot.com" size="small" />
          </div>
        </div>

        <div class="field-row field-row--actions">
          <div class="field-col field-toggles">
            <label>{{ t('settingsForm.proxyService') }}</label>
            <n-switch v-model:value="serviceOn" :loading="serviceLoading" size="small" @update:value="updateService">
              <template #checked>{{ t('settingsForm.on') }}</template>
              <template #unchecked>{{ t('settingsForm.off') }}</template>
            </n-switch>
          </div>
          <div class="field-col field-toggles">
            <label>{{ t('settingsForm.language') }}</label>
            <n-select :value="form.ui.language" :options="languageOptions" size="small" class="field-select" @update:value="handleLanguageChange" />
          </div>
        </div>

        <div class="field-actions">
          <button class="field-btn field-btn--ghost" @click="loadConfig">{{ t('settingsForm.reset') }}</button>
          <button class="field-btn field-btn--primary" :disabled="saving" @click="saveConfig">
            {{ saving ? t('settingsForm.saving') : t('settingsForm.save') }}
          </button>
        </div>
      </div>
    </n-spin>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { configApi, serviceApi, type Config } from '../api'
import { useI18n } from '../i18n'
import { useAppStore } from '../stores/app'

const emit = defineEmits<{
  (event: 'saved'): void
}>()

const appStore = useAppStore()
const message = useMessage()
const { t } = useI18n()

const form = ref<Config | null>(null)
const loading = ref(false)
const saving = ref(false)
const serviceLoading = ref(false)
const serviceOn = ref(true)

const languageOptions = [
  { label: '中文', value: 'zh' },
  { label: 'English', value: 'en' },
]

function cloneConfig(cfg: Config): Config {
  return JSON.parse(JSON.stringify(cfg)) as Config
}

async function loadConfig() {
  loading.value = true
  try {
    const cfg = await configApi.get()
    appStore.setConfig(cfg)
    form.value = cloneConfig(cfg)
    serviceOn.value = !cfg.runtime.proxy_disabled
  } catch (err) {
    message.error(err instanceof Error ? err.message : t('settingsForm.configLoadError'))
  } finally {
    loading.value = false
  }
}

async function handleLanguageChange(value: 'zh' | 'en') {
  if (!form.value) return
  form.value.ui.language = value
  appStore.language = value
  try {
    await configApi.patchUI({ language: value })
  } catch {
    // language is applied locally even if save fails
  }
}

async function updateService(enabled: boolean) {
  serviceLoading.value = true
  const previous = !enabled
  try {
    const result = await serviceApi.update(enabled)
    serviceOn.value = result.enabled
    if (form.value) form.value.runtime.proxy_disabled = !result.enabled
    if (appStore.config) appStore.config.runtime.proxy_disabled = !result.enabled
    if (appStore.status) appStore.status.service_enabled = result.enabled
    message.success(result.enabled ? t('settingsForm.serviceEnabled') : t('settingsForm.serviceDisabled'))
  } catch (err) {
    serviceOn.value = previous
    message.error(err instanceof Error ? err.message : t('settingsForm.serviceUpdateFail'))
  } finally {
    serviceLoading.value = false
  }
}

async function saveConfig() {
  if (!form.value) return
  saving.value = true
  try {
    form.value.runtime.proxy_disabled = !serviceOn.value
    await configApi.save(form.value)
    const next = cloneConfig(form.value)
    appStore.setConfig(next)
    message.success(t('settingsForm.configSaved'))
    emit('saved')
  } catch (err) {
    message.error(err instanceof Error ? err.message : t('settingsForm.configSaveFail'))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  if (appStore.config) {
    form.value = cloneConfig(appStore.config)
    serviceOn.value = !appStore.config.runtime.proxy_disabled
    return
  }
  loadConfig()
})
</script>

<style scoped>
.settings-panel {
  width: 100%;
}

.settings-fields {
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-3);
}

.field-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--cp-space-3);
}

.field-row--actions {
  grid-template-columns: 1fr 1fr;
}

.field-col {
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-1);
}

.field-col label {
  font-size: var(--cp-font-size-xs);
  font-weight: 500;
  color: var(--cp-color-text-secondary);
}

.field-number {
  width: 100%;
}

.field-select {
  width: 120px;
  flex-shrink: 0;
}

.field-toggles {
  flex-direction: row;
  align-items: center;
  gap: var(--cp-space-3);
}

.field-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--cp-space-2);
  margin-top: var(--cp-space-2);
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

@media (max-width: 640px) {
  .field-row {
    grid-template-columns: 1fr;
  }
  .field-row--actions {
    grid-template-columns: 1fr;
  }
}
</style>
