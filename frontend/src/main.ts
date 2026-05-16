type StatusResponse = {
  github_token_ready: boolean;
  copilot_token_ready: boolean;
  copilot_expires_at: string | null;
  fallback_model: string;
  config_path: string;
  base_url: string;
  service_enabled: boolean;
  active_account: Account;
};

type Config = {
  server: { host: string; port: number; read_timeout_seconds: number; write_timeout_seconds: number };
  github: Record<string, string>;
  copilot: { api_base: string; integration_id: string };
  headers: Record<string, string>;
  fallback: { preferred_prefixes: string[]; required_endpoint: string };
  keyring: { service: string; account: string };
  frontend: { enabled: boolean };
  security: { api_key: string };
  runtime: { proxy_disabled: boolean };
  auth: { active_account_id: string; accounts: Account[] };
  ui: { language: Language; theme: Theme };
};

type Account = {
  id: string;
  name: string;
  keyring_service: string;
  keyring_account: string;
  github_user_login?: string;
};

type ModelItem = {
  id: string;
  name?: string;
  vendor?: string;
  version?: string;
  family?: string;
  model_picker_enabled?: boolean;
  supported_endpoints?: string[];
  available?: boolean;
  policy?: { state?: string };
};

type StatsResponse = {
  total_requests: number;
  successful: number;
  failed: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  by_model: Record<string, { requests: number; successes: number; failures: number; total_tokens: number }>;
  recent: RequestRecord[];
};

type RequestRecord = {
  id: number;
  time: string;
  protocol: string;
  method: string;
  path: string;
  model: string;
  status: number;
  success: boolean;
  duration_ms: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  error?: string;
};

type QuotaSnapshot = {
  remaining?: number;
  quota_remaining?: number;
  entitlement?: number;
  percent_remaining?: number;
  unlimited?: boolean;
};

type QuotaResponse = {
  available: boolean;
  message?: string;
  snapshots?: Record<string, QuotaSnapshot>;
};

type Language = "zh" | "en";
type Theme = "system" | "light" | "dark";

const i18n: Record<Language, Record<string, string>> = {
  zh: {
    refresh: "刷新",
    auth: "GitHub 账号",
    switch_account: "切换账号",
    account_id: "账号 ID",
    account_name: "显示名称",
    add_account: "添加账号",
    start_auth: "开始 GitHub 授权",
    add_github_account: "添加 GitHub 账号",
    current_account: "当前账号",
    account_quota: "账号额度",
    no_accounts: "还没有已授权账号",
    legacy_account: "未识别账号",
    confirm_logout: "确认删除当前 GitHub 账号及其本机 token？",
    copy_code: "复制",
    code_copied: "授权码已复制",
    logout: "退出当前账号",
    device_code: "验证码",
    open_github: "打开 GitHub",
    dashboard: "仪表盘",
    requests: "请求",
    success: "成功",
    failed: "失败",
    tokens: "Token",
    config_path: "配置文件",
    expires_at: "Token 过期",
    quota: "额度",
    settings: "设置",
    save: "保存配置",
    service_enabled: "服务开关",
    api_key: "API 密钥",
    host: "监听主机",
    port: "端口",
    fallback_models: "回退模型优先级",
    load_models: "加载模型",
    advanced: "高级",
    usage_by_model: "模型用量",
    recent_requests: "最近请求",
    time: "时间",
    protocol: "协议",
    model: "模型",
    status: "状态",
    theme_system: "跟随系统",
    theme_light: "浅色",
    theme_dark: "深色",
    ready: "就绪",
    missing: "缺失",
    enabled: "开启",
    disabled: "关闭",
    saved: "配置已保存",
    restart_required: "部分设置重启后完全生效",
    auth_waiting: "等待 GitHub 授权",
    auth_done: "授权完成",
    logged_out: "已从系统 keyring 删除当前账号 token",
    quota_unavailable: "未发现稳定个人剩余额度接口",
    success_rate: "成功率",
    unlimited: "无限制",
    remaining: "剩余",
    chart_legend: "柱高 = 请求数 · 悬停查看 Token",
    no_usage: "暂无用量数据",
    quota_premium: "高级交互",
    quota_chat: "对话",
    quota_completions: "补全",
    fallback_hint: "从下方可用模型点 + 加入；在列表中用 ↑↓ 调整顺序，最后点确认写入配置",
    fallback_list: "回退模型列表",
    available_models: "可用模型",
    unavailable_models: "不可用模型",
    confirm_fallback: "确认修改回退模型",
    add_to_fallback: "加入回退",
    remove_from_fallback: "移除",
    fallback_saved: "回退模型已写入配置文件",
    fallback_empty: "回退列表为空，将按 Copilot 可用模型自动选择",
    move_up: "上移",
    move_down: "下移",
    request_details: "请求详情",
    method: "方法",
    path: "路径",
    duration: "耗时",
    prompt_tokens: "输入 Token",
    completion_tokens: "输出 Token",
    error: "错误",
    success_label: "成功",
    failed_label: "失败",
  },
  en: {
    refresh: "Refresh",
    auth: "GitHub accounts",
    switch_account: "Switch",
    account_id: "Account ID",
    account_name: "Display name",
    add_account: "Add account",
    start_auth: "Start GitHub auth",
    add_github_account: "Add GitHub account",
    current_account: "Current account",
    account_quota: "Account quota",
    no_accounts: "No authorized accounts yet",
    legacy_account: "Unknown account",
    confirm_logout: "Delete the current GitHub account and its local token?",
    copy_code: "Copy",
    code_copied: "Code copied",
    logout: "Logout current",
    device_code: "Code",
    open_github: "Open GitHub",
    dashboard: "Dashboard",
    requests: "Requests",
    success: "Success",
    failed: "Failed",
    tokens: "Tokens",
    config_path: "Config file",
    expires_at: "Token expiry",
    quota: "Quota",
    settings: "Settings",
    save: "Save",
    service_enabled: "Service",
    api_key: "API key",
    host: "Host",
    port: "Port",
    fallback_models: "Fallback model priority",
    load_models: "Load models",
    advanced: "Advanced",
    usage_by_model: "Usage by model",
    recent_requests: "Recent requests",
    time: "Time",
    protocol: "Protocol",
    model: "Model",
    status: "Status",
    theme_system: "System",
    theme_light: "Light",
    theme_dark: "Dark",
    ready: "ready",
    missing: "missing",
    enabled: "enabled",
    disabled: "disabled",
    saved: "Config saved",
    restart_required: "Some settings fully apply after restart",
    auth_waiting: "Waiting for GitHub auth",
    auth_done: "Authorized",
    logged_out: "Deleted current account token from system keyring",
    quota_unavailable: "No stable personal quota endpoint found",
    success_rate: "Success rate",
    unlimited: "Unlimited",
    remaining: "remaining",
    chart_legend: "Bar height = requests · hover for tokens",
    no_usage: "No usage data yet",
    quota_premium: "Premium",
    quota_chat: "Chat",
    quota_completions: "Completions",
    fallback_hint: "Add models with + below; reorder with ↑↓, then confirm to save",
    fallback_list: "Fallback list",
    available_models: "Available models",
    unavailable_models: "Unavailable models",
    confirm_fallback: "Save fallback models",
    add_to_fallback: "Add to fallback",
    remove_from_fallback: "Remove",
    fallback_saved: "Fallback models saved to config",
    fallback_empty: "Fallback list is empty; Copilot will pick any usable model",
    move_up: "Move up",
    move_down: "Move down",
    request_details: "Request details",
    method: "Method",
    path: "Path",
    duration: "Duration",
    prompt_tokens: "Prompt tokens",
    completion_tokens: "Completion tokens",
    error: "Error",
    success_label: "Success",
    failed_label: "Failed",
  },
};

const QUOTA_KEYS = ["premium_interactions", "chat", "completions"] as const;
const QUOTA_LABEL_KEYS: Record<(typeof QUOTA_KEYS)[number], string> = {
  premium_interactions: "quota_premium",
  chat: "quota_chat",
  completions: "quota_completions",
};

const $ = <T extends HTMLElement>(id: string): T => {
  const element = document.getElementById(id);
  if (!element) throw new Error(`Missing element #${id}`);
  return element as T;
};

const form = $("config-form") as HTMLFormElement;
let currentConfig: Config | null = null;
let currentLanguage: Language = "zh";
let pollTimer: number | undefined;
let availableModels: ModelItem[] = [];
let lastQuota: QuotaResponse | null = null;
let lastStats: StatsResponse | null = null;
let expandedRequestId: number | null = null;
let draftFallbackPrefixes: string[] = [];

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
    ...init,
  });
  if (!response.ok && response.status !== 202) {
    const payload = await response.json().catch(() => ({}));
    const error = typeof payload.error === "string" ? payload.error : payload.error?.message;
    throw new Error(error ?? `${response.status} ${response.statusText}`);
  }
  return response.json() as Promise<T>;
}

async function loadStatus(): Promise<void> {
  const status = await request<StatusResponse>("/api/status");
  $("base-url").textContent = status.base_url;
  $("config-path").textContent = status.config_path;
  $("expires-at").textContent = status.copilot_expires_at ?? "-";
  setPill("github-state", "GitHub", status.github_token_ready);
  setPill("copilot-state", "Copilot", status.copilot_token_ready);
  setPill("service-state", "Service", status.service_enabled, true);
}

async function loadConfig(): Promise<void> {
  currentConfig = await request<Config>("/api/config");
  currentLanguage = currentConfig.ui.language || "zh";
  applyTheme(currentConfig.ui.theme || "system");
  applyI18n(currentLanguage);
  ($("language-select") as HTMLSelectElement).value = currentLanguage;
  ($("theme-select") as HTMLSelectElement).value = currentConfig.ui.theme || "system";
  setField("server.host", currentConfig.server.host);
  setField("server.port", String(currentConfig.server.port));
  setField("security.api_key", currentConfig.security.api_key || "");
  setChecked("runtime.proxy_enabled", !currentConfig.runtime.proxy_disabled);
  setField("copilot.api_base", currentConfig.copilot.api_base);
  setField("copilot.integration_id", currentConfig.copilot.integration_id);
  const account = activeAccount(currentConfig);
  setField("keyring.service", account?.keyring_service ?? currentConfig.keyring.service);
  setField("keyring.account", account?.keyring_account ?? currentConfig.keyring.account);
  renderAccounts();
  syncDraftFromConfig();
  renderFallbackUI();
}

async function loadStats(): Promise<void> {
  const stats = await request<StatsResponse>("/api/stats");
  lastStats = stats;
  $("metric-requests").textContent = String(stats.total_requests);
  $("metric-success").textContent = String(stats.successful);
  $("metric-failed").textContent = String(stats.failed);
  $("metric-tokens").textContent = String(stats.total_tokens);
  renderSuccessRate(stats);
  renderUsage(stats);
  renderRequests(stats);
}

async function loadQuota(): Promise<void> {
  const quota = await request<QuotaResponse>("/api/quota").catch((error) => ({
    available: false,
    message: error instanceof Error ? error.message : String(error),
  }));
  lastQuota = quota;
  renderQuotaViews(quota);
}

async function loadModels(): Promise<void> {
  const payload = await request<{ data?: ModelItem[] }>("/api/models");
  availableModels = payload.data ?? [];
  syncDraftFromConfig();
  renderFallbackUI();
}

async function saveConfig(): Promise<void> {
  if (!currentConfig) return;
  const next: Config = structuredClone(currentConfig);
  next.server.host = getField("server.host");
  next.server.port = Number(getField("server.port"));
  next.security.api_key = getField("security.api_key");
  next.runtime.proxy_disabled = !getChecked("runtime.proxy_enabled");
  next.copilot.api_base = getField("copilot.api_base");
  next.copilot.integration_id = getField("copilot.integration_id");
  next.keyring.service = getField("keyring.service");
  next.keyring.account = getField("keyring.account");
  const accountIndex = next.auth.accounts.findIndex((account) => account.id === next.auth.active_account_id);
  if (accountIndex >= 0) {
    next.auth.accounts[accountIndex].keyring_service = next.keyring.service;
    next.auth.accounts[accountIndex].keyring_account = next.keyring.account;
  }
  next.ui.language = ($("language-select") as HTMLSelectElement).value as Language;
  next.ui.theme = ($("theme-select") as HTMLSelectElement).value as Theme;

  await request("/api/config", { method: "PUT", body: JSON.stringify(next) });
  await loadConfig();
  setMessage("config-message", `${t("saved")}，${t("restart_required")}`);
  await loadStatus();
}

async function updateService(enabled: boolean): Promise<void> {
  if (!currentConfig) return;
  await request("/api/service", { method: "POST", body: JSON.stringify({ enabled }) });
  currentConfig.runtime.proxy_disabled = !enabled;
  await loadStatus();
}

async function startAuth(): Promise<void> {
  const flow = await request<{ user_code: string; verification_uri: string; interval: number }>("/api/auth/device/start", {
    method: "POST",
    body: "{}",
  });
  $("device-code").textContent = flow.user_code;
  const link = $("device-link") as HTMLAnchorElement;
  link.href = flow.verification_uri;
  $("device-box").classList.remove("hidden");
  setMessage("auth-message", t("auth_waiting"));
  window.clearInterval(pollTimer);
  pollTimer = window.setInterval(() => void pollAuth(), Math.max(flow.interval, 5) * 1000);
}

async function pollAuth(): Promise<void> {
  const result = await request<{ status: string }>("/api/auth/device/poll", { method: "POST", body: "{}" });
  if (result.status === "authorized") {
    window.clearInterval(pollTimer);
    $("device-box").classList.add("hidden");
    setMessage("auth-message", t("auth_done"));
    await Promise.all([loadConfig(), loadStatus(), loadModels(), loadQuota()]);
  } else {
    setMessage("auth-message", `${t("auth_waiting")}: ${result.status}`);
  }
}

async function logout(): Promise<void> {
  if (!window.confirm(t("confirm_logout"))) return;
  await request("/api/accounts/current", { method: "DELETE" });
  setMessage("auth-message", t("logged_out"));
  await Promise.all([loadConfig(), loadStatus(), loadModels(), loadQuota()]);
}

async function switchAccount(id: string): Promise<void> {
  await request("/api/accounts/switch", { method: "POST", body: JSON.stringify({ id }) });
  await Promise.all([loadConfig(), loadStatus(), loadModels(), loadQuota()]);
}

function renderAccounts(): void {
  if (!currentConfig) return;
  const list = $("account-list");
  list.innerHTML = "";
  const activeID = currentConfig.auth.active_account_id;
  const githubAccounts = currentConfig.auth.accounts.filter((account) => account.github_user_login);
  if (githubAccounts.length === 0) {
    const empty = document.createElement("p");
    empty.className = "account-empty";
    empty.textContent = t("no_accounts");
    list.append(empty);
    return;
  }
  for (const account of githubAccounts) {
    const item = document.createElement("button");
    item.type = "button";
    item.className = `account-card${account.id === activeID ? " active" : ""}`;
    item.dataset.accountId = account.id;
    item.innerHTML = `<strong>${escapeHtml(accountLabel(account))}</strong><span>${escapeHtml(account.github_user_login ? `@${account.github_user_login}` : t("legacy_account"))}</span>`;
    if (account.id === activeID && lastQuota) {
      const quota = document.createElement("div");
      quota.className = "account-card-quota";
      renderQuotaInto(lastQuota, quota);
      item.append(quota);
    }
    list.append(item);
  }
}

function accountLabel(account?: Account): string {
  if (!account) return "-";
  return account.github_user_login || account.name || account.id || t("legacy_account");
}

function activeAccount(cfg: Config): Account | undefined {
  return cfg.auth.accounts.find((account) => account.id === cfg.auth.active_account_id) ?? cfg.auth.accounts[0];
}

function modelMatchesPref(model: ModelItem, pref: string): boolean {
  const p = pref.trim().toLowerCase();
  if (!p) return false;
  return [model.id, model.version, model.name, model.family]
    .filter((value): value is string => Boolean(value))
    .some((value) => value.toLowerCase().startsWith(p));
}

function isModelUsable(model: ModelItem): boolean {
  if (typeof model.available === "boolean") return model.available;
  const endpoint = currentConfig?.fallback.required_endpoint;
  const supportsEndpoint =
    !endpoint || !model.supported_endpoints || model.supported_endpoints.length === 0 || model.supported_endpoints.includes(endpoint);
  return model.policy?.state !== "disabled" && model.model_picker_enabled !== false && supportsEndpoint;
}

function findModelById(modelId: string): ModelItem | undefined {
  return availableModels.find((model) => model.id === modelId);
}

function findModelForPref(pref: string): ModelItem | undefined {
  const normalized = pref.trim().toLowerCase();
  return (
    availableModels.find((model) => model.id.toLowerCase() === normalized) ??
    availableModels.find((model) => modelMatchesPref(model, pref))
  );
}

function syncDraftFromConfig(): void {
  const prefs = currentConfig?.fallback.preferred_prefixes ?? [];
  draftFallbackPrefixes = [...prefs];
}

function renderFallbackUI(): void {
  renderFallbackList();
  renderModelCatalogs();
}

function renderFallbackList(): void {
  const container = $("fallback-list");
  container.innerHTML = "";
  if (draftFallbackPrefixes.length === 0) {
    const empty = document.createElement("p");
    empty.className = "fallback-empty";
    empty.textContent = t("fallback_empty");
    container.append(empty);
    return;
  }
  draftFallbackPrefixes.forEach((pref, index) => {
    const model = findModelForPref(pref) ?? { id: pref };
    container.append(buildFallbackRow(pref, model, index + 1));
  });
}

function buildFallbackRow(pref: string, model: ModelItem, rank: number): HTMLElement {
  const item = document.createElement("div");
  item.className = "fallback-item";
  item.dataset.modelId = pref;

  const order = document.createElement("span");
  order.className = "model-order";
  order.textContent = String(rank);

  const text = document.createElement("div");
  text.className = "fallback-item-text";
  const meta = [model.name, model.vendor].filter(Boolean).join(" · ");
  const detail = pref === model.id ? meta : [model.id, meta].filter(Boolean).join(" · ");
  text.innerHTML = `<strong>${escapeHtml(pref)}</strong><span>${escapeHtml(detail)}</span>`;

  const actions = document.createElement("div");
  actions.className = "model-actions";
  const up = document.createElement("button");
  up.type = "button";
  up.className = "secondary model-move";
  up.title = t("move_up");
  up.textContent = "↑";
  up.dataset.dir = "-1";
  const down = document.createElement("button");
  down.type = "button";
  down.className = "secondary model-move";
  down.title = t("move_down");
  down.textContent = "↓";
  down.dataset.dir = "1";
  const remove = document.createElement("button");
  remove.type = "button";
  remove.className = "secondary model-remove";
  remove.title = t("remove_from_fallback");
  remove.textContent = "×";
  actions.append(up, down, remove);

  item.append(order, text, actions);
  return item;
}

function renderModelCatalogs(): void {
  const availableRoot = $("available-models");
  const disabledRoot = $("disabled-models");
  const disabledPanel = $("disabled-models-panel");
  availableRoot.innerHTML = "";
  disabledRoot.innerHTML = "";

  const inDraft = new Set(draftFallbackPrefixes.map((pref) => pref.trim().toLowerCase()));
  const usable: ModelItem[] = [];
  const disabled: ModelItem[] = [];

  if (availableModels.length === 0) {
    const hint = document.createElement("p");
    hint.className = "catalog-empty";
    hint.textContent = t("load_models");
    availableRoot.append(hint);
    disabledPanel.classList.add("hidden");
    return;
  }

  for (const model of availableModels) {
    if (inDraft.has(model.id.toLowerCase())) continue;
    if (isModelUsable(model)) usable.push(model);
    else disabled.push(model);
  }

  if (usable.length === 0) {
    const hint = document.createElement("p");
    hint.className = "catalog-empty";
    hint.textContent = "—";
    availableRoot.append(hint);
  } else {
    for (const model of usable) availableRoot.append(buildCatalogRow(model, true));
  }

  disabledPanel.classList.toggle("hidden", disabled.length === 0);
  for (const model of disabled) disabledRoot.append(buildCatalogRow(model, false));
}

function buildCatalogRow(model: ModelItem, canAdd: boolean): HTMLElement {
  const row = document.createElement("div");
  row.className = `catalog-item${canAdd ? "" : " disabled"}`;
  row.dataset.modelId = model.id;

  const text = document.createElement("div");
  text.className = "catalog-item-text";
  text.innerHTML = `<strong>${escapeHtml(model.id)}</strong><span>${escapeHtml(model.name ?? "")}${model.vendor ? ` · ${escapeHtml(model.vendor)}` : ""}</span>`;

  if (canAdd) {
    const add = document.createElement("button");
    add.type = "button";
    add.className = "secondary model-add";
    add.title = t("add_to_fallback");
    add.textContent = "+";
    row.append(text, add);
  } else {
    row.append(text);
  }
  return row;
}

function addToFallback(modelId: string): void {
  if (draftFallbackPrefixes.includes(modelId)) return;
  draftFallbackPrefixes.push(modelId);
  renderFallbackUI();
}

function removeFromFallback(modelId: string): void {
  draftFallbackPrefixes = draftFallbackPrefixes.filter((id) => id !== modelId);
  renderFallbackUI();
}

function moveFallbackItem(modelId: string, delta: number): void {
  const index = draftFallbackPrefixes.indexOf(modelId);
  const target = index + delta;
  if (index < 0 || target < 0 || target >= draftFallbackPrefixes.length) return;
  const next = [...draftFallbackPrefixes];
  const [item] = next.splice(index, 1);
  next.splice(target, 0, item);
  draftFallbackPrefixes = next;
  renderFallbackUI();
}

async function confirmFallbackModels(): Promise<void> {
  const payload = await request<{ preferred_prefixes: string[]; fallback_model?: string }>("/api/fallback", {
    method: "PUT",
    body: JSON.stringify({ preferred_prefixes: draftFallbackPrefixes }),
  });
  draftFallbackPrefixes = payload.preferred_prefixes ?? draftFallbackPrefixes;
  if (currentConfig) currentConfig.fallback.preferred_prefixes = [...draftFallbackPrefixes];
  setMessage("fallback-message", t("fallback_saved"));
  renderFallbackUI();
  await loadStatus();
}

function escapeHtml(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function renderSuccessRate(stats: StatsResponse): void {
  const total = stats.total_requests;
  const rate = total > 0 ? Math.round((stats.successful / total) * 100) : 0;
  $("success-rate-label").textContent = total > 0 ? `${rate}%` : "—";
  $("success-rate-fill").style.width = total > 0 ? `${rate}%` : "0%";
}

function renderQuotaViews(quota: QuotaResponse): void {
  renderQuota(quota, "quota-visual", "quota-message");
  renderAccounts();
}

function renderQuota(quota: QuotaResponse, visualID: string, messageID: string): void {
  const visual = $(visualID);
  const message = $(messageID);
  visual.innerHTML = "";
  renderQuotaInto(quota, visual);

  if (visual.childElementCount > 0) {
    message.textContent = quota.message ?? "";
    message.classList.toggle("hidden", !quota.message);
    return;
  }

  message.textContent = quota.available ? (quota.message ?? "") : (quota.message ?? t("quota_unavailable"));
  message.classList.remove("hidden");
}

function renderQuotaInto(quota: QuotaResponse, visual: HTMLElement): void {
  visual.innerHTML = "";

  if (quota.available && quota.snapshots) {
    for (const key of QUOTA_KEYS) {
      const snap = quota.snapshots[key];
      if (!snap) continue;
      visual.append(buildQuotaBar(key, snap));
    }
  }
}

function buildQuotaBar(key: (typeof QUOTA_KEYS)[number], snap: QuotaSnapshot): HTMLElement {
  const row = document.createElement("div");
  row.className = "quota-bar";

  const head = document.createElement("div");
  head.className = "quota-bar-head";
  const label = document.createElement("span");
  label.className = "quota-bar-label";
  label.textContent = t(QUOTA_LABEL_KEYS[key]);
  const value = document.createElement("strong");
  value.className = "quota-bar-value";
  head.append(label, value);

  const track = document.createElement("div");
  track.className = "progress-track";
  const fill = document.createElement("div");
  fill.className = "progress-fill quota-fill";

  if (snap.unlimited) {
    value.textContent = t("unlimited");
    fill.style.width = "100%";
    fill.classList.add("unlimited");
    row.classList.add("is-unlimited");
  } else {
    const remaining = snap.remaining ?? snap.quota_remaining;
    const entitlement = snap.entitlement;
    let percent = snap.percent_remaining;
    if (percent === undefined && remaining !== undefined && entitlement && entitlement > 0) {
      percent = Math.min(100, Math.max(0, (remaining / entitlement) * 100));
    }
    const pct = percent ?? 0;
    fill.style.width = `${pct}%`;
    if (pct <= 15) fill.classList.add("low");
    else if (pct <= 40) fill.classList.add("mid");

    const parts: string[] = [];
    if (remaining !== undefined) parts.push(String(remaining));
    if (entitlement !== undefined && entitlement > 0) parts.push(`/${entitlement}`);
    value.textContent =
      parts.length > 0
        ? `${parts.join("")} ${t("remaining")}${percent !== undefined ? ` · ${Math.round(pct)}%` : ""}`
        : percent !== undefined
          ? `${Math.round(pct)}%`
          : "—";
  }

  track.append(fill);
  row.append(head, track);
  return row;
}

function renderUsage(stats: StatsResponse): void {
  const container = $("model-usage");
  container.innerHTML = "";
  const entries = Object.entries(stats.by_model).sort((a, b) => b[1].requests - a[1].requests);
  if (entries.length === 0) {
    const empty = document.createElement("p");
    empty.className = "chart-empty";
    empty.textContent = t("no_usage");
    container.append(empty);
    return;
  }

  const maxRequests = Math.max(...entries.map(([, usage]) => usage.requests), 1);
  const chart = document.createElement("div");
  chart.className = "bar-chart";
  chart.setAttribute("role", "img");
  chart.setAttribute("aria-label", t("usage_by_model"));

  for (const [model, usage] of entries.slice(0, 12)) {
    const height = Math.max(6, Math.round((usage.requests / maxRequests) * 100));
    const column = document.createElement("div");
    column.className = "bar-chart-column";

    const bar = document.createElement("div");
    bar.className = "bar-chart-bar";
    bar.style.height = `${height}%`;
    bar.title = `${model}\n${usage.requests} req · ${usage.total_tokens} tokens · ${usage.failures} failed`;

    const count = document.createElement("span");
    count.className = "bar-chart-count";
    count.textContent = String(usage.requests);

    const name = document.createElement("span");
    name.className = "bar-chart-label";
    name.textContent = shortModelName(model);
    name.title = model;

    column.append(count, bar, name);
    chart.append(column);
  }

  container.append(chart);
}

function shortModelName(model: string): string {
  const parts = model.split("/");
  const tail = parts[parts.length - 1] ?? model;
  return tail.length > 14 ? `${tail.slice(0, 12)}…` : tail;
}

function renderRequests(stats: StatsResponse): void {
  const body = $("request-table");
  body.innerHTML = "";
  for (const record of (stats.recent ?? []).slice(0, 80)) {
    const expanded = expandedRequestId === record.id;
    const row = document.createElement("tr");
    row.className = `request-row${record.success ? "" : " failed"}`;
    row.dataset.id = String(record.id);
    row.innerHTML = `
      <td><button type="button" class="row-toggle secondary" aria-expanded="${expanded}" title="${t("request_details")}">${expanded ? "▾" : "▸"}</button></td>
      <td>${escapeHtml(new Date(record.time).toLocaleString())}</td>
      <td>${escapeHtml(record.protocol)}</td>
      <td>${escapeHtml(record.model || "-")}</td>
      <td>${record.status}</td>
      <td>${record.total_tokens}</td>`;
    body.append(row);
    if (expanded) body.append(buildRequestDetailRow(record));
  }
}

function buildRequestDetailRow(record: RequestRecord): HTMLTableRowElement {
  const row = document.createElement("tr");
  row.className = "request-detail";
  row.dataset.id = String(record.id);
  const outcome = record.success ? t("success_label") : t("failed_label");
  const errorBlock = record.error
    ? `<div class="detail-line"><dt>${t("error")}</dt><dd>${escapeHtml(record.error)}</dd></div>`
    : "";
  row.innerHTML = `<td colspan="6">
    <div class="request-detail-panel">
      <h4>${t("request_details")} #${record.id}</h4>
      <dl class="detail-grid">
        <div class="detail-line"><dt>${t("method")}</dt><dd>${escapeHtml(record.method || "-")}</dd></div>
        <div class="detail-line"><dt>${t("path")}</dt><dd>${escapeHtml(record.path || "-")}</dd></div>
        <div class="detail-line"><dt>${t("duration")}</dt><dd>${record.duration_ms} ms</dd></div>
        <div class="detail-line"><dt>${t("status")}</dt><dd>${record.status} (${outcome})</dd></div>
        <div class="detail-line"><dt>${t("prompt_tokens")}</dt><dd>${record.prompt_tokens}</dd></div>
        <div class="detail-line"><dt>${t("completion_tokens")}</dt><dd>${record.completion_tokens}</dd></div>
        ${errorBlock}
      </dl>
    </div>
  </td>`;
  return row;
}

function toggleRequestDetail(id: number): void {
  expandedRequestId = expandedRequestId === id ? null : id;
  if (lastStats) renderRequests(lastStats);
}

function setPill(id: string, label: string, ready: boolean, service = false): void {
  const element = $(id);
  element.textContent = `${label}: ${ready ? (service ? t("enabled") : t("ready")) : service ? t("disabled") : t("missing")}`;
  element.classList.toggle("ok", ready);
  element.classList.toggle("error", !ready);
}

function setField(name: string, value: string): void {
  const input = form.elements.namedItem(name) as HTMLInputElement | null;
  if (input) input.value = value;
}

function getField(name: string): string {
  const input = form.elements.namedItem(name) as HTMLInputElement | null;
  return input?.value.trim() ?? "";
}

function setChecked(name: string, value: boolean): void {
  const input = form.elements.namedItem(name) as HTMLInputElement | null;
  if (input) input.checked = value;
}

function getChecked(name: string): boolean {
  const input = form.elements.namedItem(name) as HTMLInputElement | null;
  return input?.checked ?? false;
}

function setMessage(id: string, message: string, error = false): void {
  const element = $(id);
  element.textContent = message;
  element.classList.toggle("error", error);
}

function t(key: string): string {
  return i18n[currentLanguage][key] ?? key;
}

function applyI18n(language: Language): void {
  currentLanguage = language;
  document.documentElement.lang = language === "zh" ? "zh-CN" : "en";
  document.querySelectorAll<HTMLElement>("[data-i18n]").forEach((element) => {
    element.textContent = t(element.dataset.i18n ?? "");
  });
  document.querySelectorAll<HTMLInputElement>("[data-i18n-placeholder]").forEach((element) => {
    element.placeholder = t(element.dataset.i18nPlaceholder ?? "");
  });
}

function applyTheme(theme: Theme): void {
  document.documentElement.dataset.theme = theme === "system" ? "" : theme;
}

function wire(): void {
  $("refresh").addEventListener("click", () => void refreshAll());
  $("start-auth").addEventListener("click", () => void startAuth().catch(showAuthError));
  $("logout").addEventListener("click", () => void logout().catch(showAuthError));
  $("copy-device-code").addEventListener("click", () => void copyDeviceCode().catch(showAuthError));
  $("account-list").addEventListener("click", (event) => {
    const button = (event.target as HTMLElement).closest<HTMLButtonElement>("button.account-card");
    const id = button?.dataset.accountId;
    if (id && currentConfig?.auth.active_account_id !== id) void switchAccount(id).catch(showAuthError);
  });
  $("save-config").addEventListener("click", () => void saveConfig().catch(showConfigError));
  $("load-models").addEventListener("click", () => void loadModels().catch(showConfigError));
  $("confirm-fallback").addEventListener("click", () => void confirmFallbackModels().catch(showFallbackError));
  $("fallback-list").addEventListener("click", (event) => {
    const target = event.target as HTMLElement;
    const item = target.closest<HTMLElement>(".fallback-item");
    const modelId = item?.dataset.modelId;
    if (!modelId) return;
    const move = target.closest<HTMLButtonElement>("button.model-move");
    if (move) {
      event.preventDefault();
      moveFallbackItem(modelId, Number(move.dataset.dir));
      return;
    }
    if (target.closest("button.model-remove")) {
      event.preventDefault();
      removeFromFallback(modelId);
    }
  });
  $("available-models").addEventListener("click", (event) => {
    const add = (event.target as HTMLElement).closest<HTMLButtonElement>("button.model-add");
    if (!add) return;
    event.preventDefault();
    const modelId = add.closest<HTMLElement>(".catalog-item")?.dataset.modelId;
    if (modelId) addToFallback(modelId);
  });
  $("request-table").addEventListener("click", (event) => {
    const target = event.target as HTMLElement;
    const toggle = target.closest<HTMLButtonElement>("button.row-toggle");
    const row = target.closest<HTMLTableRowElement>("tr.request-row");
    if (!toggle && !row) return;
    const id = Number((row ?? toggle?.closest("tr"))?.dataset.id);
    if (!Number.isFinite(id)) return;
    event.preventDefault();
    toggleRequestDetail(id);
  });
  (form.elements.namedItem("runtime.proxy_enabled") as HTMLInputElement).addEventListener("change", (event) => {
    void updateService((event.currentTarget as HTMLInputElement).checked).catch(showConfigError);
  });
  $("language-select").addEventListener("change", (event) => {
    applyI18n((event.currentTarget as HTMLSelectElement).value as Language);
    renderAccounts();
    if (lastQuota) renderQuotaViews(lastQuota);
    if (lastStats) {
      renderUsage(lastStats);
      renderSuccessRate(lastStats);
    }
  });
  $("theme-select").addEventListener("change", (event) => {
    applyTheme((event.currentTarget as HTMLSelectElement).value as Theme);
  });
}

async function copyDeviceCode(): Promise<void> {
  const code = $("device-code").textContent?.trim() ?? "";
  if (!code) return;
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(code);
  } else {
    const input = document.createElement("input");
    input.value = code;
    document.body.append(input);
    input.select();
    document.execCommand("copy");
    input.remove();
  }
  setMessage("auth-message", t("code_copied"));
}

async function refreshAll(): Promise<void> {
  await Promise.all([loadConfig(), loadStatus(), loadStats(), loadQuota()]);
}

function showAuthError(error: unknown): void {
  setMessage("auth-message", error instanceof Error ? error.message : String(error), true);
}

function showConfigError(error: unknown): void {
  setMessage("config-message", error instanceof Error ? error.message : String(error), true);
}

function showFallbackError(error: unknown): void {
  setMessage("fallback-message", error instanceof Error ? error.message : String(error), true);
}

wire();
void Promise.all([loadConfig(), loadStatus(), loadStats(), loadQuota()])
  .then(() => loadModels().catch(() => undefined))
  .catch((error) => {
    showAuthError(error);
    showConfigError(error);
  });
window.setInterval(() => void loadStats().catch(() => undefined), 5000);
