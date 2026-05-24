import { describe, it, expect } from 'vitest'
import { messages } from './translations'

/**
 * Recursively collect all leaf keys from a nested object.
 * Each leaf is represented as a dot-separated path, e.g. "accountPanel.current"
 */
function collectKeys(obj: Record<string, unknown>, prefix = ''): string[] {
  const keys: string[] = []
  for (const [key, value] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      keys.push(...collectKeys(value as Record<string, unknown>, fullKey))
    } else {
      keys.push(fullKey)
    }
  }
  return keys
}

/**
 * Retrieve a value from a nested object using a dot-separated path.
 */
function getByPath(obj: Record<string, unknown>, path: string): unknown {
  let value: unknown = obj
  for (const key of path.split('.')) {
    if (value && typeof value === 'object' && key in (value as Record<string, unknown>)) {
      value = (value as Record<string, unknown>)[key]
    } else {
      return undefined
    }
  }
  return value
}

describe('translations', () => {
  it('has all top-level sections in both zh and en', () => {
    const zhKeys = Object.keys(messages.zh)
    const enKeys = Object.keys(messages.en)
    expect(zhKeys.sort()).toEqual(enKeys.sort())
  })

  it('every zh leaf key exists in en (structural parity)', () => {
    const zhLeaves = collectKeys(messages.zh)
    for (const key of zhLeaves) {
      const enValue = getByPath(messages.en as unknown as Record<string, unknown>, key)
      expect(enValue).toBeDefined()
    }
  })

  it('every en leaf key exists in zh (structural parity)', () => {
    const enLeaves = collectKeys(messages.en)
    for (const key of enLeaves) {
      const zhValue = getByPath(messages.zh as unknown as Record<string, unknown>, key)
      expect(zhValue).toBeDefined()
    }
  })

  it('all translation values are non-empty strings', () => {
    const allKeys = new Set([
      ...collectKeys(messages.zh),
      ...collectKeys(messages.en),
    ])
    for (const key of allKeys) {
      const zhValue = getByPath(messages.zh as unknown as Record<string, unknown>, key)
      const enValue = getByPath(messages.en as unknown as Record<string, unknown>, key)
      expect(typeof zhValue).toBe('string')
      expect(typeof enValue).toBe('string')
      expect((zhValue as string).length).toBeGreaterThan(0)
      expect((enValue as string).length).toBeGreaterThan(0)
    }
  })

  it('dashboardView section keys exist in both languages', () => {
    const sectionKeys = [
      'dashboardView.serviceRunning',
      'dashboardView.servicePaused',
      'dashboardView.sectionAccounts',
      'dashboardView.sectionSettings',
      'dashboardView.sectionQuota',
      'dashboardView.sectionUsage',
      'dashboardView.sectionRequests',
      'dashboardView.quotaError',
      'dashboardView.dashboardLoadError',
      'dashboardView.authComplete',
    ]
    for (const key of sectionKeys) {
      const zhVal = getByPath(messages.zh as unknown as Record<string, unknown>, key)
      const enVal = getByPath(messages.en as unknown as Record<string, unknown>, key)
      expect(zhVal).toBeDefined()
      expect(enVal).toBeDefined()
      expect(typeof zhVal).toBe('string')
      expect(typeof enVal).toBe('string')
    }
  })

  it('statusBar keys exist and have non-empty values', () => {
    const keys = [
      'statusBar.githubReady',
      'statusBar.githubMissing',
      'statusBar.copilotReady',
      'statusBar.copilotMissing',
      'statusBar.serviceOn',
      'statusBar.serviceOff',
    ]
    for (const key of keys) {
      const zhVal = getByPath(messages.zh as unknown as Record<string, unknown>, key)
      const enVal = getByPath(messages.en as unknown as Record<string, unknown>, key)
      expect(zhVal).toBeDefined()
      expect(enVal).toBeDefined()
      expect((zhVal as string).length).toBeGreaterThan(0)
      expect((enVal as string).length).toBeGreaterThan(0)
    }
  })

  it('every leaf key count matches between zh and en', () => {
    const zhLeaves = collectKeys(messages.zh)
    const enLeaves = collectKeys(messages.en)
    expect(zhLeaves.length).toBe(enLeaves.length)
  })
})
