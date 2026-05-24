<template>
  <nav class="dashboard-rail" data-testid="dashboard-rail" aria-label="Dashboard navigation">
    <button
      v-for="tab in tabs"
      :key="tab.slug"
      class="rail-tab"
      :class="{ 'is-active': activeTab === tab.slug }"
      :data-testid="`dashboard-tab-${tab.slug}`"
      :aria-label="t(tab.labelKey)"
      :title="t(tab.labelKey)"
      @click="emit('tab-change', tab.slug)"
    >
      <svg class="rail-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <template v-if="tab.slug === 'overview'">
          <path d="m3 12 9-9 9 9" />
          <path d="M5 10v10h5v-6h4v6h5V10" />
        </template>
        <template v-else-if="tab.slug === 'settings'">
          <path d="M12 15.5A3.5 3.5 0 1 0 12 8a3.5 3.5 0 0 0 0 7.5Z" />
          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.6 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 8.92 4.6 1.65 1.65 0 0 0 10 3.09V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9c.1.33.5 1 1.51 1H21a2 2 0 0 1 0 4h-.09c-1.01 0-1.41.67-1.51 1Z" />
        </template>
        <template v-else-if="tab.slug === 'diagnostics'">
          <path d="M9 12.75 11.25 15 15 9.75" />
          <path d="M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
        </template>
        <template v-else-if="tab.slug === 'usage'">
          <path d="M4 19V5" />
          <path d="M4 19h16" />
          <path d="M8 16v-5" />
          <path d="M12 16V8" />
          <path d="M16 16v-3" />
        </template>
        <template v-else>
          <path d="M7 3h7l5 5v13H7z" />
          <path d="M14 3v5h5" />
          <path d="M9 13h6" />
          <path d="M9 17h6" />
        </template>
      </svg>
      <span class="rail-label">{{ t(tab.labelKey) }}</span>
    </button>
  </nav>
</template>

<script setup lang="ts">
import { useI18n } from '../i18n'
import { tabs, type DashboardTab } from '../views/dashboard-tabs'

defineProps<{
  activeTab: DashboardTab
}>()

const emit = defineEmits<{
  'tab-change': [tab: DashboardTab]
}>()

const { t } = useI18n()
</script>

<style scoped>
.dashboard-rail {
  position: sticky;
  top: var(--cp-space-6);
  z-index: 1;
  display: flex;
  flex: 0 0 140px;
  flex-direction: column;
  gap: var(--cp-space-2);
  width: 140px;
  padding: var(--cp-space-2);
  border: 1px solid var(--cp-color-border);
  border-radius: var(--cp-radius-lg);
  background: var(--cp-color-surface);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  overflow: hidden;
  box-shadow: var(--cp-shadow-card);
  transition: box-shadow var(--cp-transition-med);
}

.rail-tab {
  display: flex;
  align-items: center;
  gap: var(--cp-space-3);
  width: 100%;
  min-width: 46px;
  height: 46px;
  padding: 0 var(--cp-space-3);
  border: 1px solid transparent;
  border-radius: var(--cp-radius-md);
  background: transparent;
  color: var(--cp-color-text-secondary);
  cursor: pointer;
  outline: none;
  transition: all var(--cp-transition-fast);
}

.rail-tab:hover,
.rail-tab:focus-visible {
  border-color: var(--cp-color-border);
  background: var(--cp-color-primary-soft);
  color: var(--cp-color-primary);
}

.rail-tab.is-active {
  border-color: var(--cp-color-primary);
  background: var(--cp-color-primary-soft);
  color: var(--cp-color-primary);
}

.rail-icon {
  flex: 0 0 20px;
  width: 20px;
  height: 20px;
}

.rail-label {
  overflow: hidden;
  font-size: var(--cp-font-size-sm);
  font-weight: 600;
  white-space: nowrap;
}
</style>
