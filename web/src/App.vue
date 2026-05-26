<template>
  <n-config-provider :theme="naiveTheme" :locale="naiveLocale" :date-locale="naiveDateLocale">
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
import { zhCN, dateZhCN, enUS, dateEnUS, darkTheme } from 'naive-ui'
import { useAppStore } from './stores/app'

const appStore = useAppStore()

const naiveTheme = computed(() => {
  return appStore.resolvedTheme === 'dark' ? darkTheme : null
})

const naiveLocale = computed(() => {
  return appStore.language === 'en' ? enUS : zhCN
})

const naiveDateLocale = computed(() => {
  return appStore.language === 'en' ? dateEnUS : dateZhCN
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
  --cp-font-size-xs: 15px;
  --cp-font-size-sm: 16px;
  --cp-font-size-md: 18px;
  --cp-font-size-lg: 21px;
  --cp-font-size-xl: 27px;
  --cp-font-size-2xl: 35px;

  --cp-color-bg: #f4f7fb;
  --cp-color-surface: rgba(255, 255, 255, 0.24);
  --cp-color-card: rgba(255, 255, 255, 0.32);
  --cp-color-border: rgba(255, 255, 255, 0.26);
  --cp-color-text: #1a1a2e;
  --cp-color-text-secondary: rgba(26, 26, 46, 0.78);
  --cp-color-text-muted: rgba(26, 26, 46, 0.55);
  --cp-color-primary: #1d4ed8;
  --cp-color-primary-soft: rgba(37, 99, 235, 0.12);
  --cp-color-primary-hover: #1e40af;
  --cp-color-success: #18a058;
  --cp-color-success-soft: rgba(24, 160, 88, 0.12);
  --cp-color-warning: #f0a020;
  --cp-color-warning-soft: rgba(240, 160, 32, 0.14);
  --cp-color-error: #d03050;
  --cp-color-error-soft: rgba(208, 48, 80, 0.12);
  --cp-shadow-card: 0 4px 24px rgba(28, 45, 78, 0.10);
  --cp-shadow-float: 0 12px 48px rgba(20, 30, 54, 0.15);
  --cp-text-shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.12);
  --cp-text-shadow-md: 0 2px 4px rgba(0, 0, 0, 0.18);
  --cp-text-shadow-lg: 0 3px 8px rgba(0, 0, 0, 0.22);
  --cp-text-outline:
     1px 0 0 rgba(0,0,0,0.10), -1px 0 0 rgba(0,0,0,0.10),
     0 1px 0 rgba(0,0,0,0.10), 0 -1px 0 rgba(0,0,0,0.10);
  --cp-transition-fast: 160ms ease;
  --cp-transition-med: 300ms cubic-bezier(0.4, 0, 0.2, 1);
  --cp-wallpaper-url: url('/background/light.png');
}

:root[data-theme='dark'] {
  --cp-color-bg: #080e1a;
  --cp-color-surface: rgba(18, 26, 44, 0.22);
  --cp-color-card: rgba(18, 26, 44, 0.30);
  --cp-color-border: rgba(255, 255, 255, 0.12);
  --cp-color-text: rgba(245, 247, 250, 0.96);
  --cp-color-text-secondary: rgba(245, 247, 250, 0.75);
  --cp-color-text-muted: rgba(245, 247, 250, 0.52);
  --cp-color-primary: #8bb9fe;
  --cp-color-primary-soft: rgba(110, 168, 254, 0.14);
  --cp-color-primary-hover: #a8ccff;
  --cp-color-success-soft: rgba(24, 160, 88, 0.16);
  --cp-color-warning-soft: rgba(240, 160, 32, 0.16);
  --cp-color-error-soft: rgba(208, 48, 80, 0.16);
  --cp-shadow-card: 0 4px 24px rgba(0, 0, 0, 0.26);
  --cp-shadow-float: 0 16px 56px rgba(0, 0, 0, 0.42);
  --cp-text-shadow-sm: 0 1px 3px rgba(0, 0, 0, 0.35);
  --cp-text-shadow-md: 0 2px 6px rgba(0, 0, 0, 0.45);
  --cp-text-shadow-lg: 0 4px 10px rgba(0, 0, 0, 0.55);
  --cp-text-outline:
     1px 0 0 rgba(0,0,0,0.20), -1px 0 0 rgba(0,0,0,0.20),
     0 1px 0 rgba(0,0,0,0.20), 0 -1px 0 rgba(0,0,0,0.20);
  --cp-wallpaper-url: url('/background/dark.png');
}

html, body, #app {
  margin: 0;
  padding: 0;
  height: 100%;
  overflow: hidden;
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'SF Pro Display', 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
  font-weight: 650;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  background: var(--cp-color-bg);
  color: var(--cp-color-text);
  text-shadow: var(--cp-text-shadow-sm);
  transition: background var(--cp-transition-med), color var(--cp-transition-med);
}

body {
  min-height: 100%;
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
