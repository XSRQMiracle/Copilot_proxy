import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import DashboardView from './DashboardView.vue'

// Mock naive-ui before any component imports
vi.mock('naive-ui', () => ({
  useMessage: () => ({ error: vi.fn(), success: vi.fn() }),
}))

// Mock the API module so no real HTTP requests are made
vi.mock('../api', () => ({
  configApi: {
    get: vi.fn().mockResolvedValue({
      server: { host: '0.0.0.0', port: 15432, read_timeout_seconds: 120, write_timeout_seconds: 120 },
      github: {},
      copilot: { api_base: 'https://api.githubcopilot.com', integration_id: 'v2' },
      headers: {},
      security: { api_key: '', admin_password: '' },
      runtime: { proxy_disabled: false },
      auth: { active_account_id: '', accounts: [] },
      ui: { language: 'zh', theme: 'system' },
    }),
  },
  quotaApi: {
    get: vi.fn().mockResolvedValue({ available: false }),
  },
  statsApi: {
    get: vi.fn().mockResolvedValue({
      total_requests: 0, successful: 0, failed: 0,
      prompt_tokens: 0, completion_tokens: 0, total_tokens: 0,
      by_model: {}, recent: [],
    }),
  },
  statusApi: {
    get: vi.fn().mockResolvedValue({
      github_token_ready: true, copilot_token_ready: true,
      copilot_expires_at: null, config_path: '', base_url: 'http://localhost:15432',
      service_enabled: true, active_account: '',
    }),
  },
}))

vi.mock('../stores/app', () => ({
  useAppStore: () => ({
    config: null,
    status: { base_url: 'http://localhost:15432', service_enabled: true, copilot_token_ready: true, github_token_ready: true, active_account: '' },
    stats: null,
    quota: null,
    isLoggedIn: false,
    theme: 'system',
    language: 'zh',
    activeAccountName: '',
    serviceEnabled: true,
    copilotReady: true,
    githubReady: true,
    resolvedTheme: 'light',
    setConfig: vi.fn(),
    applyTheme: vi.fn(),
    cycleTheme: vi.fn(),
  }),
}))

vi.mock('../i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

describe('DashboardView', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }))
  })

  function createWrapper() {
    return mount(DashboardView, {
      global: {
        stubs: {
          // Stub the shell to render its slot with a wrapper
          DashboardShell: {
            template: '<div data-testid="dashboard-shell"><slot /></div>',
          },
          DashboardRail: {
            template: `
              <nav data-testid="dashboard-rail">
                <button
                  v-for="tab in tabs"
                  :key="tab.slug"
                  :data-testid="'dashboard-tab-' + tab.slug"
                  :class="{ 'is-active': activeTab === tab.slug }"
                  @click="$emit('tab-change', tab.slug)"
                >{{ tab.label }}</button>
              </nav>`,
            props: ['activeTab'],
            emits: ['tab-change'],
            data() {
              return {
                tabs: [
                  { slug: 'overview', label: '总览' },
                  { slug: 'settings', label: '设置' },
                  { slug: 'diagnostics', label: '测试' },
                  { slug: 'usage', label: '用量' },
                  { slug: 'logs', label: '日志' },
                ],
              }
            },
          },
          DeviceAuth: { template: '<div />' },
          DashboardCards: { template: '<div />' },
          QuotaDisplay: { template: '<div />' },
          UsageChart: { template: '<div />' },
          RequestTable: { template: '<div />' },
          AccountPanel: { template: '<div />' },
          SettingsForm: { template: '<div />' },
          ConnectionBanner: { template: '<div />' },
          ThemeToggle: { template: '<div />' },
          NAlert: { template: '<div><slot /></div>' },
        },
      },
    })
  }

  it('renders the dashboard shell', async () => {
    const wrapper = createWrapper()
    await new Promise(process.nextTick)

    expect(wrapper.find('[data-testid="dashboard-shell"]').exists()).toBe(true)
  })

  it('renders the dashboard rail', async () => {
    const wrapper = createWrapper()
    await new Promise(process.nextTick)

    expect(wrapper.find('[data-testid="dashboard-rail"]').exists()).toBe(true)
  })

  it('renders five tab buttons with correct testids', async () => {
    const wrapper = createWrapper()
    await new Promise(process.nextTick)

    const tabButtons = wrapper.findAll('[data-testid^="dashboard-tab-"]')
    expect(tabButtons.length).toBe(5)
  })

  it('has correct tab labels', async () => {
    const wrapper = createWrapper()
    await new Promise(process.nextTick)

    const tabButtons = wrapper.findAll('[data-testid^="dashboard-tab-"]')
    const expectedSlugs = ['overview', 'settings', 'diagnostics', 'usage', 'logs']
    tabButtons.forEach((btn, i) => {
      expect(btn.attributes('data-testid')).toBe(`dashboard-tab-${expectedSlugs[i]}`)
    })
  })

  it('clicks a tab to change active state', async () => {
    const wrapper = createWrapper()
    await new Promise(process.nextTick)

    // Click the "settings" tab
    const settingsTab = wrapper.find('[data-testid="dashboard-tab-settings"]')
    await settingsTab.trigger('click')
    // Wait for Vue reactivity
    await new Promise(process.nextTick)

    // The settings tab should now have the is-active class
    expect(settingsTab.classes()).toContain('is-active')
  })
})
