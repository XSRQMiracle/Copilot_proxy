import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import UsageChart from './UsageChart.vue'
import type { StatsResponse } from '../api'

const state = vi.hoisted(() => ({
  stats: null as StatsResponse | null,
}))

vi.mock('../stores/app', () => ({
  useAppStore: () => ({
    stats: state.stats,
  }),
}))

vi.mock('../i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (!params) return key
      return `${key}:${Object.values(params).join(',')}`
    },
  }),
}))

describe('UsageChart', () => {
  beforeEach(() => {
    state.stats = null
  })

  it('renders empty state without stats', () => {
    const wrapper = mount(UsageChart)
    expect(wrapper.text()).toContain('usageChart.empty')
  })

  it('renders summary, trend, and model rows with reasoning tokens', () => {
    state.stats = {
      total_requests: 2,
      successful: 2,
      failed: 0,
      prompt_tokens: 30,
      completion_tokens: 20,
      reasoning_tokens: 7,
      total_tokens: 50,
      by_model: {
        'gpt-4.1': {
          requests: 2,
          successes: 2,
          failures: 0,
          prompt_tokens: 30,
          completion_tokens: 20,
          reasoning_tokens: 7,
          total_tokens: 50,
        },
      },
      recent: [
        {
          id: 1,
          time: new Date('2026-05-25T10:15:00Z').toISOString(),
          protocol: 'openai',
          method: 'POST',
          path: '/v1/chat/completions',
          model: 'gpt-4.1',
          status: 200,
          success: true,
          duration_ms: 120,
          prompt_tokens: 10,
          completion_tokens: 8,
          reasoning_tokens: 3,
          total_tokens: 18,
        },
        {
          id: 2,
          time: new Date('2026-05-25T11:15:00Z').toISOString(),
          protocol: 'openai',
          method: 'POST',
          path: '/v1/chat/completions',
          model: 'gpt-4.1',
          status: 200,
          success: true,
          duration_ms: 140,
          prompt_tokens: 20,
          completion_tokens: 12,
          reasoning_tokens: 4,
          total_tokens: 32,
        },
      ],
    }

    const wrapper = mount(UsageChart)
    expect(wrapper.text()).toContain('usageChart.totalTokens')
    expect(wrapper.text()).toContain('usageChart.reasoningTokens')
    expect(wrapper.text()).toContain('gpt-4.1')
    expect(wrapper.find('svg.usage-trend').exists()).toBe(true)
  })
})
