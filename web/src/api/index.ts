const BASE = ''

export interface DeviceFlow {
  user_code: string
  verification_uri: string
  expires_in: number
  interval: number
}

export interface Account {
  id: string
  name: string
  github_user_login?: string
  github_token?: string
}

export interface Config {
  server: { host: string; port: number; read_timeout_seconds: number; write_timeout_seconds: number }
  github: Record<string, string>
  copilot: { api_base: string; integration_id: string }
  headers: Record<string, string>
  fallback: { preferred_prefixes: string[]; required_endpoint: string }
  security: { api_key: string; admin_password?: string }
  runtime: { proxy_disabled: boolean }
  auth: { active_account_id: string; accounts: Account[] }
  ui: { language: string; theme: string }
}

export interface StatusResponse {
  github_token_ready: boolean
  copilot_token_ready: boolean
  copilot_expires_at: string | null
  fallback_model: string
  config_path: string
  base_url: string
  service_enabled: boolean
  active_account: string
}

export interface ModelItem {
  id: string
  name?: string
  vendor?: string
  available?: boolean
  policy?: { state?: string }
  model_picker_enabled?: boolean
  supported_endpoints?: string[]
}

export interface QuotaSnapshot {
  remaining?: number
  quota_remaining?: number
  entitlement?: number
  percent_remaining?: number
  unlimited?: boolean
}

export interface QuotaResponse {
  available: boolean
  message?: string
  snapshots?: Record<string, QuotaSnapshot>
}

export interface RequestRecord {
  id: number
  time: string
  protocol: string
  method: string
  path: string
  model: string
  status: number
  success: boolean
  duration_ms: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  error?: string
}

export interface ModelUsage {
  requests: number
  successes: number
  failures: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
}

export interface StatsResponse {
  total_requests: number
  successful: number
  failed: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  by_model: Record<string, ModelUsage>
  recent: RequestRecord[]
}

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, {
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init,
  })
  if (res.status === 202) {
    return { status: 'pending' } as unknown as T
  }
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error((data as any).error ?? res.statusText)
  }
  return res.json() as Promise<T>
}

export function setAuthToken(token: string) {
  localStorage.setItem('admin_token', token)
}

export function getAuthToken(): string | null {
  return localStorage.getItem('admin_token')
}

function authHeaders(): Record<string, string> {
  const token = getAuthToken()
  if (token) {
    return { 'Authorization': `Bearer ${token}` }
  }
  return {}
}

export const authApi = {
  login: (password: string) => {
    const headers = { ...authHeaders() }
    return api<{ status: string; token: string }>('/api/auth/login', {
      method: 'POST',
      headers,
      body: JSON.stringify({ password }),
    })
  },
}

export const accountsApi = {
  list: () => api<{ active_account_id: string; accounts: Account[] }>('/api/accounts', {
    headers: authHeaders(),
  }),
  add: (githubToken: string) => api<Account>('/api/accounts', {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ github_token: githubToken }),
  }),
  remove: (id: string) => api<void>(`/api/accounts/${id}`, {
    method: 'DELETE',
    headers: authHeaders(),
  }),
  switch: (id: string) => api<{ active_account_id: string }>('/api/accounts/switch', {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ id }),
  }),
}

export const deviceApi = {
  start: () => api<DeviceFlow>('/api/auth/device/start', {
    method: 'POST',
    headers: authHeaders(),
    body: '{}',
  }),
  poll: () => api<{ status: string }>('/api/auth/device/poll', {
    method: 'POST',
    headers: authHeaders(),
    body: '{}',
  }),
}

export const configApi = {
  get: () => api<Config>('/api/config', { headers: authHeaders() }),
  save: (cfg: Config) => api<{ status: string }>('/api/config', {
    method: 'PUT',
    headers: authHeaders(),
    body: JSON.stringify(cfg),
  }),
}

export const statusApi = {
  get: () => api<StatusResponse>('/api/status', { headers: authHeaders() }),
}

export const statsApi = {
  get: () => api<StatsResponse>('/api/stats', { headers: authHeaders() }),
}

export const modelsApi = {
  list: () => api<{ data?: ModelItem[] }>('/api/models', { headers: authHeaders() }),
}

export const quotaApi = {
  get: () => api<QuotaResponse>('/api/quota', { headers: authHeaders() }),
}

export const fallbackApi = {
  update: (prefixes: string[]) => api<{ status: string; preferred_prefixes: string[]; fallback_model?: string }>('/api/fallback', {
    method: 'PUT',
    headers: authHeaders(),
    body: JSON.stringify({ preferred_prefixes: prefixes }),
  }),
}

export const serviceApi = {
  update: (enabled: boolean) => api<{ enabled: boolean }>('/api/service', {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ enabled }),
  }),
}
