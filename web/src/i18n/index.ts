import { useAppStore } from '../stores/app'
import { messages, type Language } from './translations'

export function useI18n() {
  const appStore = useAppStore()

  function t(key: string, params?: Record<string, string | number>): string {
    const keys = key.split('.')
    const lang = (appStore.language || 'zh') as Language
    let value: unknown = messages[lang]
    for (const k of keys) {
      if (value && typeof value === 'object' && k in value) {
        value = (value as Record<string, unknown>)[k]
      } else {
        return key
      }
    }
    if (typeof value === 'string') {
      if (params) {
        let result = value
        for (const [k, v] of Object.entries(params)) {
          result = result.replace(`{${k}}`, String(v))
        }
        return result
      }
      return value
    }
    return key
  }

  return { t }
}
