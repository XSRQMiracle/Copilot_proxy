<template>
  <button
    class="theme-cycle"
    :title="themeLabel"
    @click="appStore.cycleTheme()"
  >
    <svg v-if="appStore.theme === 'system'" class="theme-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <rect x="2" y="3" width="20" height="14" rx="2" ry="2" />
      <line x1="8" y1="21" x2="16" y2="21" />
      <line x1="12" y1="17" x2="12" y2="21" />
      <circle cx="12" cy="10" r="3" opacity="0.5" />
    </svg>
    <svg v-else-if="appStore.theme === 'light'" class="theme-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="12" cy="12" r="5" />
      <line x1="12" y1="1" x2="12" y2="3" />
      <line x1="12" y1="21" x2="12" y2="23" />
      <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
      <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
      <line x1="1" y1="12" x2="3" y2="12" />
      <line x1="21" y1="12" x2="23" y2="12" />
      <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
      <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
    </svg>
    <svg v-else class="theme-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
    </svg>
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '../stores/app'

const appStore = useAppStore()

const themeLabel = computed(() => {
  const labels = { system: '跟随系统', light: '浅色模式', dark: '深色模式' }
  return labels[appStore.theme]
})
</script>

<style scoped>
.theme-cycle {
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
  outline: none;
}

.theme-cycle:hover {
  border-color: var(--cp-color-primary);
  color: var(--cp-color-primary);
  background: var(--cp-color-primary-soft);
  transform: scale(1.05);
}

.theme-cycle:active {
  transform: scale(0.95);
}

.theme-icon {
  width: 18px;
  height: 18px;
}
</style>
