<template>
  <main class="login-shell">
    <section class="login-orbit" aria-hidden="true" />
    <n-card class="login-card" :bordered="false">
      <n-space vertical :size="24">
        <header class="login-header">
          <n-tag type="primary" round>Copilot Proxy</n-tag>
          <h1>{{ t('loginView.title') }}</h1>
          <p>{{ t('loginView.desc') }}</p>
        </header>

        <n-form @submit.prevent="handleLogin">
          <n-form-item :label="t('loginView.passwordLabel')">
            <n-input
              v-model:value="password"
              type="password"
              size="large"
              show-password-on="click"
              :placeholder="t('loginView.passwordPlaceholder')"
              :disabled="loading"
              @keyup.enter="handleLogin"
            />
          </n-form-item>

          <n-alert v-if="error" type="error" :show-icon="true" class="login-error">
            {{ error }}
          </n-alert>

          <n-button
            type="primary"
            size="large"
            block
            attr-type="submit"
            :loading="loading"
            :disabled="!password.trim()"
          >
            {{ t('loginView.submit') }}
          </n-button>
        </n-form>
      </n-space>
    </n-card>
  </main>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { authApi, setAuthToken } from '../api'
import { useI18n } from '../i18n'
import { useAppStore } from '../stores/app'

const router = useRouter()
const message = useMessage()
const appStore = useAppStore()
const { t } = useI18n()

const password = ref('')
const loading = ref(false)
const error = ref('')

async function handleLogin() {
  const trimmed = password.value.trim()
  if (!trimmed || loading.value) return

  loading.value = true
  error.value = ''
  try {
    const result = await authApi.login(trimmed)
    setAuthToken(result.token)
    appStore.isLoggedIn = true
    message.success(t('loginView.loginSuccess'))
    await router.push('/')
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('loginView.loginFail')
    message.error(error.value)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-shell {
  position: relative;
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: var(--cp-space-8);
  overflow: hidden;
  background:
    radial-gradient(circle at 20% 18%, var(--cp-color-primary-soft), transparent 32%),
    radial-gradient(circle at 82% 72%, var(--cp-color-success-soft), transparent 30%),
    var(--cp-color-bg);
}

.login-orbit {
  position: absolute;
  width: min(62vw, 760px);
  aspect-ratio: 1;
  border: 1px solid var(--cp-color-border);
  border-radius: 50%;
  transform: rotate(-18deg);
  box-shadow: inset 0 0 0 var(--cp-space-8) rgba(255, 255, 255, 0.02);
}

.login-card {
  width: min(100%, 430px);
  position: relative;
  z-index: 1;
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-lg);
  box-shadow: var(--cp-shadow-float);
  background: var(--cp-color-surface);
  backdrop-filter: blur(18px);
}

.login-header {
  display: grid;
  gap: var(--cp-space-3);
}

.login-header h1 {
  margin: 0;
  font-size: var(--cp-font-size-2xl);
  letter-spacing: -0.04em;
}

.login-header p {
  margin: 0;
  color: var(--cp-color-text-muted);
  line-height: 1.7;
}

.login-error {
  margin-bottom: var(--cp-space-4);
}
</style>
