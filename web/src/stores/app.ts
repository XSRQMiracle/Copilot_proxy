import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Config, StatusResponse, StatsResponse, QuotaResponse } from '../api'

const themeOrder = ['system', 'light', 'dark'] as const
let mediaQuery: MediaQueryList | null = null
let mediaListener: (() => void) | null = null

function resolveTheme(t: 'system' | 'light' | 'dark'): 'light' | 'dark' {
  if (t === 'system') {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }
  return t
}

function applyDataTheme(resolved: 'light' | 'dark') {
  if (resolved === 'dark') {
    document.documentElement.setAttribute('data-theme', 'dark')
  } else {
    document.documentElement.removeAttribute('data-theme')
  }
}

export const useAppStore = defineStore('app', () => {
  const config = ref<Config | null>(null)
  const status = ref<StatusResponse | null>(null)
  const stats = ref<StatsResponse | null>(null)
  const quota = ref<QuotaResponse | null>(null)
  const isLoggedIn = ref(false)
  const theme = ref<'system' | 'light' | 'dark'>('system')
  const language = ref<'zh' | 'en'>('zh')

  const activeAccountName = computed(() => status.value?.active_account ?? '')
  const serviceEnabled = computed(() => status.value?.service_enabled ?? true)
  const copilotReady = computed(() => status.value?.copilot_token_ready ?? false)
  const githubReady = computed(() => status.value?.github_token_ready ?? false)

  /** Resolved theme for consumers — always 'light' or 'dark' */
  const resolvedTheme = computed(() => resolveTheme(theme.value))

  function setConfig(cfg: Config) {
    config.value = cfg
    applyTheme((cfg.ui.theme as 'system' | 'light' | 'dark') || 'system')
    language.value = (cfg.ui.language as 'zh' | 'en') || 'zh'
  }

  function applyTheme(t: 'system' | 'light' | 'dark') {
    theme.value = t
    localStorage.setItem('theme', t)
    applyDataTheme(resolveTheme(t))

    // Set up / tear down system preference listener
    if (t === 'system') {
      if (!mediaQuery) {
        mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
        mediaListener = () => applyDataTheme(resolveTheme('system'))
        mediaQuery.addEventListener('change', mediaListener)
      }
    } else {
      if (mediaQuery && mediaListener) {
        mediaQuery.removeEventListener('change', mediaListener)
        mediaQuery = null
        mediaListener = null
      }
    }
  }

  function cycleTheme() {
    const idx = themeOrder.indexOf(theme.value)
    const next = themeOrder[(idx + 1) % themeOrder.length]
    applyTheme(next)
  }

  // Restore persisted theme on init; always apply so login page gets the right theme
  const saved = localStorage.getItem('theme') as 'system' | 'light' | 'dark' | null
  if (saved && ['system', 'light', 'dark'].includes(saved)) {
    applyTheme(saved)
  } else {
    applyTheme('system')
  }

  return {
    config, status, stats, quota,
    isLoggedIn, theme, language, resolvedTheme,
    activeAccountName, serviceEnabled, copilotReady, githubReady,
    setConfig, applyTheme, cycleTheme,
  }
})
