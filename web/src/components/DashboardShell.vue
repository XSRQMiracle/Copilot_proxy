<template>
  <main class="dashboard-shell" data-testid="dashboard-shell">
    <div class="shell-container">
      <ConnectionBanner :connected="connected" />

      <header class="shell-header">
        <div class="shell-title-group">
          <span class="shell-kicker">Dashboard</span>
          <h1 class="shell-title">Copilot Proxy</h1>
        </div>

        <div class="shell-status-area">
          <div class="status-tags">
            <span class="status-tag" :class="githubReady ? 'tag-ok' : 'tag-warn'">
              <span class="status-dot" />
              {{ githubReady ? t('statusBar.githubReady') : t('statusBar.githubMissing') }}
            </span>
            <span class="status-tag" :class="copilotReady ? 'tag-ok' : 'tag-warn'">
              <span class="status-dot" />
              {{ copilotReady ? t('statusBar.copilotReady') : t('statusBar.copilotMissing') }}
            </span>
            <span class="status-tag" :class="serviceEnabled ? 'tag-ok' : 'tag-err'">
              <span class="status-dot" />
              {{ serviceEnabled ? t('statusBar.serviceOn') : t('statusBar.serviceOff') }}
            </span>
          </div>

          <div class="shell-actions">
            <span v-if="baseUrl" class="status-url">{{ baseUrl }}</span>
            <button class="shell-refresh" :class="{ 'is-spinning': refreshing }" :title="t('dashboardView.serviceRunning')" @click="emit('refresh')">
              <svg class="refresh-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="23 4 23 10 17 10" />
                <polyline points="1 20 1 14 7 14" />
                <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" />
              </svg>
            </button>
            <ThemeToggle />
          </div>
        </div>
      </header>

      <div class="shell-layout">
        <slot />
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../i18n'
import { useAppStore } from '../stores/app'
import ConnectionBanner from './ConnectionBanner.vue'
import ThemeToggle from './ThemeToggle.vue'

defineProps<{
  connected: boolean
  refreshing: boolean
}>()

const emit = defineEmits<{
  refresh: []
}>()

const appStore = useAppStore()
const { t } = useI18n()

const githubReady = computed(() => appStore.githubReady)
const copilotReady = computed(() => appStore.copilotReady)
const serviceEnabled = computed(() => appStore.serviceEnabled)
const baseUrl = computed(() => appStore.status?.base_url ?? '')
</script>

<style scoped>
.dashboard-shell {
  min-height: 100vh;
  padding: var(--cp-space-6);
  background-color: var(--cp-color-bg);
  background-image:
    radial-gradient(ellipse 80% 60% at 0% 20%, var(--cp-color-primary-soft) 0%, transparent 60%),
    radial-gradient(ellipse 60% 50% at 100% 80%, var(--cp-color-warning-soft) 0%, transparent 60%),
    var(--cp-wallpaper-url);
  background-size: auto, auto, cover;
  background-position: 0 0, 0 0, center;
  background-repeat: repeat, repeat, no-repeat;
  background-attachment: scroll, scroll, fixed;
}

.shell-container {
  width: 78%;
  max-width: 1440px;
  margin: 0 auto;
}

.shell-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--cp-space-6);
  margin: var(--cp-space-4) 0 var(--cp-space-6);
  padding: var(--cp-space-4) var(--cp-space-5);
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-lg);
  background: var(--cp-color-surface);
  backdrop-filter: blur(10px) saturate(150%) contrast(0.95);
  -webkit-backdrop-filter: blur(10px) saturate(150%) contrast(0.95);
  transition: background var(--cp-transition-med);
}

.shell-title-group {
  min-width: 0;
}

.shell-kicker {
  display: block;
  margin-bottom: var(--cp-space-1);
  font-size: var(--cp-font-size-xs);
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--cp-color-primary);
  text-shadow: var(--cp-text-shadow-sm);
}

.shell-title {
  margin: 0;
  font-size: clamp(var(--cp-font-size-xl), 2.5vw, 36px);
  font-weight: 700;
  letter-spacing: -0.03em;
  color: var(--cp-color-text);
  text-shadow: var(--cp-text-shadow-md), var(--cp-text-outline);
}

.shell-status-area,
.shell-actions,
.status-tags {
  display: flex;
  align-items: center;
}

.shell-status-area {
  gap: var(--cp-space-4);
  min-width: 0;
}

.status-tags {
  gap: var(--cp-space-2);
  flex-wrap: wrap;
  justify-content: flex-end;
}

.status-tag {
  display: inline-flex;
  align-items: center;
  gap: var(--cp-space-1);
  padding: var(--cp-space-1) var(--cp-space-3);
  font-size: var(--cp-font-size-xs);
  font-weight: 500;
  border: 1px solid var(--cp-color-border);
  border-radius: 999px;
  color: var(--cp-color-text-secondary);
  text-shadow: var(--cp-text-shadow-md);
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.tag-ok .status-dot {
  background: var(--cp-color-success);
}

.tag-warn .status-dot {
  background: var(--cp-color-warning);
}

.tag-err .status-dot {
  background: var(--cp-color-error);
}

.shell-actions {
  gap: var(--cp-space-3);
  flex-shrink: 0;
}

.status-url {
  max-width: 220px;
  overflow: hidden;
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  font-size: var(--cp-font-size-xs);
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--cp-color-text-muted);
  text-shadow: var(--cp-text-shadow-sm);
}

.shell-refresh {
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
  outline: none;
  transition: all var(--cp-transition-fast);
}

.shell-refresh:hover {
  border-color: var(--cp-color-primary);
  background: var(--cp-color-primary-soft);
  color: var(--cp-color-primary);
}

.shell-refresh.is-spinning .refresh-icon {
  animation: spin 0.6s linear infinite;
}

.refresh-icon {
  width: 16px;
  height: 16px;
}

.shell-layout {
  display: flex;
  gap: var(--cp-space-5);
  align-items: flex-start;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 1280px) {
  .shell-container {
    width: 92%;
  }
}

@media (max-width: 1024px) {
  .dashboard-shell {
    padding: var(--cp-space-4);
  }

  .shell-container {
    width: 100%;
  }

  .shell-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .shell-status-area {
    width: 100%;
    justify-content: space-between;
  }
}

@media (max-width: 720px) {
  .shell-status-area {
    align-items: flex-start;
    flex-direction: column;
  }

  .status-tags {
    justify-content: flex-start;
  }

  .shell-layout {
    gap: var(--cp-space-3);
  }
}
</style>
