<template>
  <main class="dashboard-shell">
    <ConnectionBanner :connected="connected" />
    <StatusBar />

    <header class="dashboard-hero">
      <div class="hero-left">
        <span class="hero-badge">Dashboard</span>
        <h1 class="hero-title">Copilot Proxy</h1>
      </div>
      <div class="hero-right">
        <span class="hero-dot" :class="appStore.serviceEnabled ? 'dot-on' : 'dot-off'" />
        <span class="hero-status">{{ appStore.serviceEnabled ? t('dashboardView.serviceRunning') : t('dashboardView.servicePaused') }}</span>
        <button class="hero-refresh" :class="{ 'is-spinning': refreshing }" @click="refreshAll">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="23 4 23 10 17 10" />
            <polyline points="1 20 1 14 7 14" />
            <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" />
          </svg>
        </button>
      </div>
    </header>

    <n-alert v-if="loadError" type="error" closable @close="loadError = ''">
      {{ loadError }}
    </n-alert>

    <div class="dashboard-body">
      <aside class="dash-left">
        <div class="dash-card">
          <div class="dash-card-header">
            <h2>{{ t('dashboardView.sectionAccounts') }}</h2>
            <button class="dash-card-btn" @click="showDeviceAuth = true">+ GitHub</button>
          </div>
          <div class="dash-card-body">
            <AccountPanel ref="accountPanelRef" @start-auth="showDeviceAuth = true" />
          </div>
        </div>

        <div class="dash-card">
          <div class="dash-card-header">
            <h2>{{ t('dashboardView.sectionSettings') }}</h2>
          </div>
          <div class="dash-card-body">
            <SettingsForm @saved="refreshAll" />
          </div>
        </div>

        <div class="dash-card">
          <div class="dash-card-header">
            <h2>{{ t('dashboardView.sectionFallback') }}</h2>
          </div>
          <div class="dash-card-body">
            <ModelPicker />
          </div>
        </div>
      </aside>

      <section class="dash-right">
        <div class="dash-card">
          <div class="dash-card-body">
            <DashboardCards />
          </div>
        </div>

        <div class="dash-card">
          <div class="dash-card-header">
            <h2>{{ t('dashboardView.sectionQuota') }}</h2>
          </div>
          <div class="dash-card-body">
            <QuotaDisplay />
          </div>
        </div>

        <div class="dash-card">
          <div class="dash-card-header">
            <h2>{{ t('dashboardView.sectionUsage') }}</h2>
          </div>
          <div class="dash-card-body">
            <UsageChart />
          </div>
        </div>
      </section>
    </div>

    <div class="dash-card dash-table-section">
      <div class="dash-card-header">
        <h2>{{ t('dashboardView.sectionRequests') }}</h2>
      </div>
      <div class="dash-card-body">
        <RequestTable />
      </div>
    </div>

    <DeviceAuth v-model:show="showDeviceAuth" @authorized="handleAuthorized" />
  </main>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { configApi, quotaApi, statsApi, statusApi } from '../api'
import { useI18n } from '../i18n'
import { useAppStore } from '../stores/app'
import ConnectionBanner from '../components/ConnectionBanner.vue'
import StatusBar from '../components/StatusBar.vue'
import AccountPanel from '../components/AccountPanel.vue'
import DeviceAuth from '../components/DeviceAuth.vue'
import DashboardCards from '../components/DashboardCards.vue'
import QuotaDisplay from '../components/QuotaDisplay.vue'
import UsageChart from '../components/UsageChart.vue'
import SettingsForm from '../components/SettingsForm.vue'
import ModelPicker from '../components/ModelPicker.vue'
import RequestTable from '../components/RequestTable.vue'

const appStore = useAppStore()
const message = useMessage()
const { t } = useI18n()

const refreshing = ref(false)
const loadError = ref('')
const showDeviceAuth = ref(false)
const connected = ref(true)
const accountPanelRef = ref<{ refresh: () => Promise<void> } | null>(null)
let statsTimer: number | undefined
let pingTimer: number | undefined

async function refreshStats() {
  appStore.stats = await statsApi.get()
}

async function refreshAll() {
  if (refreshing.value) return
  refreshing.value = true
  loadError.value = ''
  try {
    const [status, config, stats, quota] = await Promise.all([
      statusApi.get(),
      configApi.get(),
      statsApi.get(),
      quotaApi.get().catch((err) => {
        const reason = err instanceof Error ? err.message : t('dashboardView.quotaError')
        return { available: false, message: reason }
      }),
    ])

    appStore.status = status
    appStore.setConfig(config)
    appStore.stats = stats
    appStore.quota = quota
    appStore.isLoggedIn = true
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : t('dashboardView.dashboardLoadError')
    message.error(loadError.value)
  } finally {
    refreshing.value = false
  }
}

async function ping() {
  try {
    const res = await fetch('/')
    connected.value = res.ok
  } catch {
    connected.value = false
  }
}

function startPing() {
  ping()
  pingTimer = window.setInterval(ping, 3000)
}

async function handleAuthorized() {
  message.success(t('dashboardView.authComplete'))
  await accountPanelRef.value?.refresh()
  await refreshAll()
}

onMounted(async () => {
  await refreshAll()
  statsTimer = window.setInterval(() => {
    refreshStats().catch(() => {})
  }, 5000)
  startPing()
})

onUnmounted(() => {
  if (statsTimer) window.clearInterval(statsTimer)
  if (pingTimer) window.clearInterval(pingTimer)
})
</script>

<style scoped>
.dashboard-shell {
  min-height: 100vh;
  padding: var(--cp-space-6);
  background:
    radial-gradient(ellipse 80% 60% at 0% 20%, var(--cp-color-primary-soft) 0%, transparent 60%),
    radial-gradient(ellipse 60% 50% at 100% 80%, var(--cp-color-warning-soft) 0%, transparent 60%),
    var(--cp-color-bg);
}

/* Hero */
.dashboard-hero {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin: var(--cp-space-4) 0 var(--cp-space-6);
}

.hero-left {
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-2);
}

.hero-badge {
  font-size: var(--cp-font-size-xs);
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--cp-color-primary);
}

.hero-title {
  margin: 0;
  font-size: clamp(var(--cp-font-size-xl), 2.5vw, 36px);
  font-weight: 700;
  letter-spacing: -0.03em;
  color: var(--cp-color-text);
}

.hero-right {
  display: flex;
  align-items: center;
  gap: var(--cp-space-2);
}

.hero-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  transition: background var(--cp-transition-med);
}

.dot-on {
  background: var(--cp-color-success);
  box-shadow: 0 0 8px var(--cp-color-success);
}

.dot-off {
  background: var(--cp-color-error);
  box-shadow: 0 0 8px var(--cp-color-error);
}

.hero-status {
  font-size: var(--cp-font-size-sm);
  color: var(--cp-color-text-secondary);
}

.hero-refresh {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-sm);
  background: var(--cp-color-surface);
  color: var(--cp-color-text);
  cursor: pointer;
  transition: all var(--cp-transition-fast);
  margin-left: var(--cp-space-2);
  outline: none;
}

.hero-refresh:hover {
  border-color: var(--cp-color-primary);
  color: var(--cp-color-primary);
  background: var(--cp-color-primary-soft);
}

.hero-refresh.is-spinning svg {
  animation: spin 0.6s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* Body: left-right layout */
.dashboard-body {
  display: flex;
  gap: var(--cp-space-5);
  align-items: start;
}

.dash-left {
  flex: 0 0 34%;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-5);
}

.dash-right {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-5);
}

/* Unified card component */
.dash-card {
  background: var(--cp-color-surface);
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-lg);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  overflow: hidden;
  transition: box-shadow var(--cp-transition-med);
}

.dash-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--cp-space-4) var(--cp-space-5) 0;
}

.dash-card-header h2 {
  margin: 0;
  font-size: var(--cp-font-size-sm);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--cp-color-text-muted);
}

.dash-card-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: var(--cp-space-1) var(--cp-space-3);
  font-size: var(--cp-font-size-xs);
  font-weight: 500;
  border: 1px solid var(--cp-color-primary);
  border-radius: var(--cp-radius-sm);
  background: transparent;
  color: var(--cp-color-primary);
  cursor: pointer;
  transition: all var(--cp-transition-fast);
  outline: none;
}

.dash-card-btn:hover {
  background: var(--cp-color-primary-soft);
}

.dash-card-body {
  padding: var(--cp-space-4) var(--cp-space-5) var(--cp-space-5);
}

.dash-table-section {
  margin-top: var(--cp-space-5);
}

/* Responsive: stack on narrow screens */
@media (max-width: 1024px) {
  .dashboard-body {
    flex-direction: column;
  }

  .dash-left,
  .dash-right {
    flex: 1;
    width: 100%;
  }
}

@media (max-width: 640px) {
  .dashboard-shell {
    padding: var(--cp-space-4);
  }

  .dashboard-hero {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--cp-space-3);
  }
}
</style>
