<template>
  <div class="account-panel">
    <n-spin :show="loading">
      <div v-if="accounts.length" class="account-list">
        <article v-for="account in accounts" :key="account.id" class="account-item">
          <div class="account-name">
            {{ account.name || account.github_user_login || account.id }}
          </div>

          <div class="account-actions">
            <button
              v-if="account.id !== activeAccountId"
              class="account-btn account-btn-switch"
              :disabled="switchingId === account.id"
              @click.stop="switchAccount(account.id)"
            >
              {{ switchingId === account.id ? '…' : t('accountPanel.switch') }}
            </button>
            <button v-else class="account-btn account-btn-current" disabled>
              {{ t('accountPanel.current') }}
            </button>

            <button class="account-btn account-btn-del" @click.stop="confirmDelete(account)">
              {{ t('accountPanel.delete') }}
            </button>
          </div>
        </article>
      </div>

      <div v-else class="account-empty">
        <p>{{ t('accountPanel.noAccount') }}</p>
        <button class="account-add-btn" @click="emit('start-auth')">{{ t('accountPanel.startAuth') }}</button>
      </div>
    </n-spin>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useDialog, useMessage } from 'naive-ui'
import { accountsApi, statusApi, type Account } from '../api'
import { useI18n } from '../i18n'
import { useAppStore } from '../stores/app'

const emit = defineEmits<{
  (event: 'start-auth'): void
  (event: 'switched'): void
}>()

const appStore = useAppStore()
const message = useMessage()
const dialog = useDialog()
const { t } = useI18n()

const accounts = ref<Account[]>([])
const activeAccountId = ref('')
const loading = ref(false)
const switchingId = ref('')

async function refresh() {
  loading.value = true
  try {
    const result = await accountsApi.list()
    accounts.value = result.accounts
    activeAccountId.value = result.active_account_id
    if (appStore.config) {
      appStore.config.auth.accounts = result.accounts
      appStore.config.auth.active_account_id = result.active_account_id
    }
    appStore.status = await statusApi.get().catch(() => appStore.status)
  } catch (err) {
    message.error(err instanceof Error ? err.message : t('accountPanel.loadError'))
  } finally {
    loading.value = false
  }
}

async function switchAccount(id: string) {
  switchingId.value = id
  try {
    const result = await accountsApi.switch(id)
    activeAccountId.value = result.active_account_id
    if (appStore.config) appStore.config.auth.active_account_id = result.active_account_id
    appStore.status = await statusApi.get()
    message.success(t('accountPanel.switchSuccess'))
    emit('switched')
  } catch (err) {
    message.error(err instanceof Error ? err.message : t('accountPanel.switchFail'))
  } finally {
    switchingId.value = ''
  }
}

function confirmDelete(account: Account) {
  dialog.warning({
    title: t('accountPanel.deleteTitle'),
    content: t('accountPanel.deleteConfirm', { name: account.name || account.github_user_login || account.id }),
    positiveText: t('accountPanel.deletePositive'),
    negativeText: t('accountPanel.deleteNegative'),
    draggable: true,
    onPositiveClick: async () => {
      try {
        await accountsApi.remove(account.id)
        message.success(t('accountPanel.deleteSuccess'))
        await refresh()
      } catch (err) {
        message.error(err instanceof Error ? err.message : t('accountPanel.deleteFail'))
        return false
      }
    },
  })
}

onMounted(refresh)

defineExpose({ refresh })
</script>

<style scoped>
.account-panel {
  width: 100%;
  max-height: 360px;
  overflow-y: auto;
  overflow-x: hidden;
}

.account-list {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--cp-space-3);
}

.account-item {
  width: 150px;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-3);
  padding: var(--cp-space-3);
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-md);
  background: var(--cp-color-primary-soft);
}

.account-name {
  min-height: 38px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--cp-color-text);
  font-size: var(--cp-font-size-sm);
  font-weight: 600;
  line-height: 1.35;
  text-align: center;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.account-actions {
  display: flex;
  justify-content: space-between;
  gap: 4px;
}

.account-btn {
  width: 58px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  font-size: var(--cp-font-size-xs);
  border-radius: var(--cp-radius-sm);
  cursor: pointer;
  border: 1px solid transparent;
  transition: all var(--cp-transition-fast);
  outline: none;
  white-space: nowrap;
}

.account-btn-switch {
  background: var(--cp-color-primary-soft);
  color: var(--cp-color-primary);
  border-color: var(--cp-color-primary);
}

.account-btn-switch:hover {
  box-shadow: 0 0 8px var(--cp-color-primary-soft);
}

.account-btn-switch:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.account-btn-current {
  background: var(--cp-color-border);
  color: var(--cp-color-text-muted);
  border-color: var(--cp-color-border);
  cursor: default;
}

.account-btn-current:disabled {
  opacity: 1;
}

.account-btn-del {
  background: transparent;
  color: var(--cp-color-error);
  border-color: var(--cp-color-error);
}

.account-btn-del:hover {
  background: var(--cp-color-error-soft);
  box-shadow: 0 0 8px var(--cp-color-error-soft);
}

.account-empty {
  text-align: center;
  padding: var(--cp-space-8) 0;
}

.account-empty p {
  margin: 0 0 var(--cp-space-4);
  color: var(--cp-color-text-muted);
  font-size: var(--cp-font-size-sm);
}

.account-add-btn {
  padding: var(--cp-space-2) var(--cp-space-5);
  font-size: var(--cp-font-size-sm);
  border: 1px solid var(--cp-color-primary);
  border-radius: var(--cp-radius-sm);
  background: transparent;
  color: var(--cp-color-primary);
  cursor: pointer;
  transition: all var(--cp-transition-fast);
  outline: none;
}

.account-add-btn:hover {
  background: var(--cp-color-primary-soft);
}
</style>
