<template>
  <n-config-provider :theme="naiveTheme" :locale="zhCN" :date-locale="dateZhCN">
    <n-message-provider>
      <n-dialog-provider>
        <n-notification-provider>
          <n-loading-bar-provider>
            <router-view />
          </n-loading-bar-provider>
        </n-notification-provider>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { zhCN, dateZhCN, darkTheme } from 'naive-ui'
import { useAppStore } from './stores/app'

const appStore = useAppStore()

const naiveTheme = computed(() => {
  return appStore.resolvedTheme === 'dark' ? darkTheme : null
})
</script>

<style>
:root {
  --cp-space-1: 4px;
  --cp-space-2: 8px;
  --cp-space-3: 12px;
  --cp-space-4: 16px;
  --cp-space-5: 20px;
  --cp-space-6: 24px;
  --cp-space-8: 32px;
  --cp-space-10: 40px;
  --cp-radius-sm: 8px;
  --cp-radius-md: 12px;
  --cp-radius-lg: 18px;
  --cp-font-size-xs: 12px;
  --cp-font-size-sm: 13px;
  --cp-font-size-md: 15px;
  --cp-font-size-lg: 18px;
  --cp-font-size-xl: 24px;
  --cp-font-size-2xl: 32px;

  --cp-color-bg: #f4f7fb;
  --cp-color-surface: rgba(255, 255, 255, 0.86);
  --cp-color-card: #ffffff;
  --cp-color-border: rgba(23, 35, 61, 0.1);
  --cp-color-text: #1f2937;
  --cp-color-text-secondary: rgba(31, 41, 55, 0.72);
  --cp-color-text-muted: rgba(31, 41, 55, 0.5);
  --cp-color-primary: #2563eb;
  --cp-color-primary-soft: rgba(37, 99, 235, 0.1);
  --cp-color-primary-hover: #1d4ed8;
  --cp-color-success: #18a058;
  --cp-color-success-soft: rgba(24, 160, 88, 0.1);
  --cp-color-warning: #f0a020;
  --cp-color-warning-soft: rgba(240, 160, 32, 0.12);
  --cp-color-error: #d03050;
  --cp-color-error-soft: rgba(208, 48, 80, 0.1);
  --cp-shadow-card: 0 2px 16px rgba(28, 45, 78, 0.06);
  --cp-shadow-float: 0 12px 48px rgba(20, 30, 54, 0.12);
  --cp-transition-fast: 160ms ease;
  --cp-transition-med: 300ms cubic-bezier(0.4, 0, 0.2, 1);
}

:root[data-theme='dark'] {
  --cp-color-bg: #080e1a;
  --cp-color-surface: rgba(18, 26, 44, 0.82);
  --cp-color-card: #111b30;
  --cp-color-border: rgba(255, 255, 255, 0.08);
  --cp-color-text: rgba(235, 238, 245, 0.94);
  --cp-color-text-secondary: rgba(235, 238, 245, 0.7);
  --cp-color-text-muted: rgba(235, 238, 245, 0.48);
  --cp-color-primary: #6ea8fe;
  --cp-color-primary-soft: rgba(110, 168, 254, 0.12);
  --cp-color-primary-hover: #8bb9fe;
  --cp-color-success-soft: rgba(24, 160, 88, 0.15);
  --cp-color-warning-soft: rgba(240, 160, 32, 0.15);
  --cp-color-error-soft: rgba(208, 48, 80, 0.15);
  --cp-shadow-card: 0 2px 20px rgba(0, 0, 0, 0.28);
  --cp-shadow-float: 0 16px 56px rgba(0, 0, 0, 0.36);
}

html, body, #app {
  margin: 0;
  padding: 0;
  height: 100%;
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'SF Pro Display', 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  background: var(--cp-color-bg);
  color: var(--cp-color-text);
  transition: background var(--cp-transition-med), color var(--cp-transition-med);
}

* {
  box-sizing: border-box;
}

/* Smooth scrollbar */
::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}

::-webkit-scrollbar-track {
  background: transparent;
}

::-webkit-scrollbar-thumb {
  background: var(--cp-color-border);
  border-radius: 3px;
}

::-webkit-scrollbar-thumb:hover {
  background: var(--cp-color-text-muted);
}
</style>
