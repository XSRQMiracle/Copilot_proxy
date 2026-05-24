export type DashboardTab = 'overview' | 'settings' | 'diagnostics' | 'usage' | 'logs'

export interface TabDefinition {
  slug: DashboardTab
  labelKey: string
  icon: string
}

export const tabs: TabDefinition[] = [
  { slug: 'overview', labelKey: 'dashboardTabs.overview', icon: '' },
  { slug: 'settings', labelKey: 'dashboardTabs.settings', icon: '' },
  { slug: 'diagnostics', labelKey: 'dashboardTabs.dialogTest', icon: '' },
  { slug: 'usage', labelKey: 'dashboardTabs.usage', icon: '' },
  { slug: 'logs', labelKey: 'dashboardTabs.logs', icon: '' },
]
