<template>
  <div class="status-bar">
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
    <div class="status-right">
      <span v-if="baseUrl" class="status-url">{{ baseUrl }}</span>
      <ThemeToggle />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../i18n'
import { useAppStore } from '../stores/app'
import ThemeToggle from './ThemeToggle.vue'

const appStore = useAppStore()
const { t } = useI18n()

const githubReady = computed(() => appStore.githubReady)
const copilotReady = computed(() => appStore.copilotReady)
const serviceEnabled = computed(() => appStore.serviceEnabled)
const baseUrl = computed(() => appStore.status?.base_url ?? '')
</script>

<style scoped>
.status-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--cp-space-3) 0;
  border-bottom: 1px solid var(--cp-color-border);
  margin-bottom: var(--cp-space-4);
}

.status-tags {
  display: flex;
  gap: var(--cp-space-3);
}

.status-tag {
  display: inline-flex;
  align-items: center;
  gap: var(--cp-space-1);
  padding: var(--cp-space-1) var(--cp-space-3);
  font-size: var(--cp-font-size-xs);
  font-weight: 500;
  border-radius: 999px;
  border: 1px solid var(--cp-color-border);
  color: var(--cp-color-text-secondary);
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

.status-right {
  display: flex;
  align-items: center;
  gap: var(--cp-space-3);
}

.status-url {
  font-size: var(--cp-font-size-xs);
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  color: var(--cp-color-text-muted);
}
</style>
