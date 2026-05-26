<template>
  <DashboardShell :connected="connected" :refreshing="refreshing" @refresh="refreshAll">
    <DashboardRail :active-tab="activeTab" @tab-change="activeTab = $event" />

    <section class="dashboard-content" data-testid="dashboard-content">
      <n-alert v-if="loadError" type="error" closable @close="loadError = ''">
        {{ loadError }}
      </n-alert>

      <template v-if="activeTab === 'overview'">
        <div class="dashboard-grid">
          <div class="dash-card span-full">
            <div class="dash-card-body">
              <DashboardCards />
            </div>
          </div>

          <div class="dash-card">
            <div class="dash-card-header">
              <h2>{{ t('dashboardView.sectionAccounts') }}</h2>
              <button class="dash-card-btn" @click="showDeviceAuth = true">+ GitHub</button>
            </div>
            <div class="dash-card-body">
              <AccountPanel ref="accountPanelRef" @start-auth="showDeviceAuth = true" @switched="refreshAll" />
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
        </div>
      </template>

      <template v-else-if="activeTab === 'settings'">
        <div class="dashboard-grid settings-grid">
          <div class="dash-card span-full">
            <div class="dash-card-header">
              <h2>{{ t('dashboardView.sectionSettings') }}</h2>
            </div>
            <div class="dash-card-body">
              <SettingsForm @saved="refreshAll" />
            </div>
          </div>
        </div>
      </template>

      <template v-else-if="activeTab === 'diagnostics'">
        <ChatTestPanel />
      </template>

      <template v-else-if="activeTab === 'usage'">
        <div class="dashboard-grid">
          <div class="dash-card span-full">
            <div class="dash-card-header">
              <h2>{{ t('dashboardView.sectionUsage') }}</h2>
            </div>
            <div class="dash-card-body">
              <UsageChart />
            </div>
          </div>
        </div>
      </template>

      <template v-else>
        <div class="dash-card">
          <div class="dash-card-header">
            <h2>{{ t('dashboardView.sectionRequests') }}</h2>
          </div>
          <div class="dash-card-body logs-scroll-container">
            <RequestTable />
          </div>
        </div>
      </template>
    </section>

    <DeviceAuth v-model:show="showDeviceAuth" @authorized="handleAuthorized" />
  </DashboardShell>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { configApi, quotaApi, statsApi, statusApi } from '../api'
import { useI18n } from '../i18n'
import { useAppStore } from '../stores/app'
import AccountPanel from '../components/AccountPanel.vue'
import DashboardCards from '../components/DashboardCards.vue'
import { type DashboardTab } from '../views/dashboard-tabs'
import DashboardRail from '../components/DashboardRail.vue'
import DashboardShell from '../components/DashboardShell.vue'
import DeviceAuth from '../components/DeviceAuth.vue'
import ChatTestPanel from '../components/ChatTestPanel.vue'
import QuotaDisplay from '../components/QuotaDisplay.vue'
import RequestTable from '../components/RequestTable.vue'
import SettingsForm from '../components/SettingsForm.vue'
import UsageChart from '../components/UsageChart.vue'

const appStore = useAppStore()
const message = useMessage()
const { t } = useI18n()

const activeTab = ref<DashboardTab>('overview')
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
.dashboard-content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: var(--cp-space-5);
  overflow-y: auto;
}

.dashboard-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: var(--cp-space-5);
}

.settings-grid {
  align-items: start;
}

.span-full {
  grid-column: 1 / -1;
}

.dash-card {
  background: var(--cp-color-surface);
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-lg);
  backdrop-filter: blur(10px) saturate(150%) contrast(0.95);
  -webkit-backdrop-filter: blur(10px) saturate(150%) contrast(0.95);
  overflow: hidden;
  transition: box-shadow var(--cp-transition-med), background var(--cp-transition-med);
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
  text-shadow: var(--cp-text-shadow-sm), var(--cp-text-outline);
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
  text-shadow: var(--cp-text-shadow-sm);
  cursor: pointer;
  outline: none;
  transition: all var(--cp-transition-fast);
}

.dash-card-btn:hover {
  background: var(--cp-color-primary-soft);
}

.dash-card-body {
  padding: var(--cp-space-4) var(--cp-space-5) var(--cp-space-5);
}

.logs-scroll-container {
  max-height: 65vh;
  overflow-y: auto;
}

.placeholder-body {
  min-height: 220px;
  color: var(--cp-color-text-secondary);
  text-shadow: var(--cp-text-shadow-sm);
}

.placeholder-body p {
  margin: 0;
}

@media (max-width: 1024px) {
  .dashboard-grid {
    grid-template-columns: 1fr;
  }
}
</style>
