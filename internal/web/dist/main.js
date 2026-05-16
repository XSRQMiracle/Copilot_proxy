"use strict";
const i18n = {
    zh: {
        refresh: "刷新",
        auth: "授权",
        switch_account: "切换账号",
        account_id: "账号 ID",
        account_name: "显示名称",
        add_account: "添加账号",
        start_auth: "开始 GitHub 授权",
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
    },
    en: {
        refresh: "Refresh",
        auth: "Auth",
        switch_account: "Switch",
        account_id: "Account ID",
        account_name: "Display name",
        add_account: "Add account",
        start_auth: "Start GitHub auth",
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
    },
};
const $ = (id) => {
    const element = document.getElementById(id);
    if (!element)
        throw new Error(`Missing element #${id}`);
    return element;
};
const form = $("config-form");
let currentConfig = null;
let currentLanguage = "zh";
let pollTimer;
let availableModels = [];
async function request(url, init) {
    const response = await fetch(url, {
        headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
        ...init,
    });
    if (!response.ok && response.status !== 202) {
        const payload = await response.json().catch(() => ({}));
        const error = typeof payload.error === "string" ? payload.error : payload.error?.message;
        throw new Error(error ?? `${response.status} ${response.statusText}`);
    }
    return response.json();
}
async function loadStatus() {
    const status = await request("/api/status");
    $("base-url").textContent = status.base_url;
    $("config-path").textContent = status.config_path;
    $("expires-at").textContent = status.copilot_expires_at ?? "-";
    setPill("github-state", "GitHub", status.github_token_ready);
    setPill("copilot-state", "Copilot", status.copilot_token_ready);
    setPill("service-state", "Service", status.service_enabled, true);
}
async function loadConfig() {
    currentConfig = await request("/api/config");
    currentLanguage = currentConfig.ui.language || "zh";
    applyTheme(currentConfig.ui.theme || "system");
    applyI18n(currentLanguage);
    $("language-select").value = currentLanguage;
    $("theme-select").value = currentConfig.ui.theme || "system";
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
    renderModelList();
}
async function loadStats() {
    const stats = await request("/api/stats");
    $("metric-requests").textContent = String(stats.total_requests);
    $("metric-success").textContent = String(stats.successful);
    $("metric-failed").textContent = String(stats.failed);
    $("metric-tokens").textContent = String(stats.total_tokens);
    renderUsage(stats);
    renderRequests(stats);
}
async function loadQuota() {
    const quota = await request("/api/quota").catch((error) => ({
        available: false,
        message: error instanceof Error ? error.message : String(error),
    }));
    $("quota-summary").textContent = quota.available ? quota.message : t("quota_unavailable");
}
async function loadModels() {
    const payload = await request("/api/models");
    availableModels = payload.data ?? [];
    renderModelList();
}
async function saveConfig() {
    if (!currentConfig)
        return;
    const next = structuredClone(currentConfig);
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
    next.fallback.preferred_prefixes = selectedFallbackModels();
    next.ui.language = $("language-select").value;
    next.ui.theme = $("theme-select").value;
    await request("/api/config", { method: "PUT", body: JSON.stringify(next) });
    currentConfig = next;
    setMessage("config-message", `${t("saved")}，${t("restart_required")}`);
    await loadStatus();
}
async function updateService(enabled) {
    if (!currentConfig)
        return;
    await request("/api/service", { method: "POST", body: JSON.stringify({ enabled }) });
    currentConfig.runtime.proxy_disabled = !enabled;
    await loadStatus();
}
async function startAuth() {
    const flow = await request("/api/auth/device/start", {
        method: "POST",
        body: "{}",
    });
    $("device-code").textContent = flow.user_code;
    const link = $("device-link");
    link.href = flow.verification_uri;
    $("device-box").classList.remove("hidden");
    setMessage("auth-message", t("auth_waiting"));
    window.clearInterval(pollTimer);
    pollTimer = window.setInterval(() => void pollAuth(), Math.max(flow.interval, 5) * 1000);
}
async function pollAuth() {
    const result = await request("/api/auth/device/poll", { method: "POST", body: "{}" });
    if (result.status === "authorized") {
        window.clearInterval(pollTimer);
        $("device-box").classList.add("hidden");
        setMessage("auth-message", t("auth_done"));
        await Promise.all([loadStatus(), loadModels(), loadQuota()]);
    }
    else {
        setMessage("auth-message", `${t("auth_waiting")}: ${result.status}`);
    }
}
async function logout() {
    await request("/api/auth/logout", { method: "POST", body: "{}" });
    setMessage("auth-message", t("logged_out"));
    await loadStatus();
}
async function addAccount() {
    await request("/api/accounts", {
        method: "POST",
        body: JSON.stringify({ id: $("new-account-id").value.trim(), name: $("new-account-name").value.trim() }),
    });
    $("new-account-id").value = "";
    $("new-account-name").value = "";
    await loadConfig();
}
async function switchAccount() {
    const id = $("account-select").value;
    await request("/api/accounts/switch", { method: "POST", body: JSON.stringify({ id }) });
    await Promise.all([loadConfig(), loadStatus(), loadModels(), loadQuota()]);
}
function renderAccounts() {
    if (!currentConfig)
        return;
    const select = $("account-select");
    select.innerHTML = "";
    for (const account of currentConfig.auth.accounts) {
        const option = document.createElement("option");
        option.value = account.id;
        option.textContent = `${account.name} (${account.id})`;
        select.append(option);
    }
    select.value = currentConfig.auth.active_account_id;
}
function activeAccount(cfg) {
    return cfg.auth.accounts.find((account) => account.id === cfg.auth.active_account_id) ?? cfg.auth.accounts[0];
}
function renderModelList() {
    const container = $("model-list");
    container.innerHTML = "";
    const selected = new Set(currentConfig?.fallback.preferred_prefixes ?? []);
    const fallbackModels = (currentConfig?.fallback.preferred_prefixes ?? []).map((id) => ({ id }));
    const models = availableModels.length > 0 ? availableModels : fallbackModels;
    for (const model of models) {
        const enabled = model.policy?.state !== "disabled" && model.model_picker_enabled !== false;
        const item = document.createElement("label");
        item.className = `model-option ${enabled ? "" : "disabled"}`;
        const checkbox = document.createElement("input");
        checkbox.type = "checkbox";
        checkbox.value = model.id;
        checkbox.checked = selected.has(model.id);
        item.append(checkbox);
        const text = document.createElement("div");
        text.innerHTML = `<strong>${model.id}</strong><span>${model.name ?? ""} ${model.vendor ? `· ${model.vendor}` : ""}</span>`;
        item.append(text);
        container.append(item);
    }
}
function renderUsage(stats) {
    const container = $("model-usage");
    container.innerHTML = "";
    const entries = Object.entries(stats.by_model).sort((a, b) => b[1].requests - a[1].requests);
    for (const [model, usage] of entries) {
        const item = document.createElement("div");
        item.innerHTML = `<strong>${model}</strong><span>${usage.requests} req · ${usage.total_tokens} tokens · ${usage.failures} failed</span>`;
        container.append(item);
    }
    if (entries.length === 0)
        container.textContent = "-";
}
function renderRequests(stats) {
    const body = $("request-table");
    body.innerHTML = "";
    for (const record of (stats.recent ?? []).slice(0, 80)) {
        const row = document.createElement("tr");
        row.innerHTML = `<td>${new Date(record.time).toLocaleTimeString()}</td><td>${record.protocol}</td><td>${record.model || "-"}</td><td>${record.status}</td><td>${record.total_tokens}</td>`;
        body.append(row);
    }
}
function selectedFallbackModels() {
    return Array.from(document.querySelectorAll("#model-list input:checked")).map((input) => input.value);
}
function setPill(id, label, ready, service = false) {
    const element = $(id);
    element.textContent = `${label}: ${ready ? (service ? t("enabled") : t("ready")) : service ? t("disabled") : t("missing")}`;
    element.classList.toggle("ok", ready);
    element.classList.toggle("error", !ready);
}
function setField(name, value) {
    const input = form.elements.namedItem(name);
    if (input)
        input.value = value;
}
function getField(name) {
    const input = form.elements.namedItem(name);
    return input?.value.trim() ?? "";
}
function setChecked(name, value) {
    const input = form.elements.namedItem(name);
    if (input)
        input.checked = value;
}
function getChecked(name) {
    const input = form.elements.namedItem(name);
    return input?.checked ?? false;
}
function setMessage(id, message, error = false) {
    const element = $(id);
    element.textContent = message;
    element.classList.toggle("error", error);
}
function t(key) {
    return i18n[currentLanguage][key] ?? key;
}
function applyI18n(language) {
    currentLanguage = language;
    document.documentElement.lang = language === "zh" ? "zh-CN" : "en";
    document.querySelectorAll("[data-i18n]").forEach((element) => {
        element.textContent = t(element.dataset.i18n ?? "");
    });
    document.querySelectorAll("[data-i18n-placeholder]").forEach((element) => {
        element.placeholder = t(element.dataset.i18nPlaceholder ?? "");
    });
}
function applyTheme(theme) {
    document.documentElement.dataset.theme = theme === "system" ? "" : theme;
}
function wire() {
    $("refresh").addEventListener("click", () => void refreshAll());
    $("start-auth").addEventListener("click", () => void startAuth().catch(showAuthError));
    $("logout").addEventListener("click", () => void logout().catch(showAuthError));
    $("add-account").addEventListener("click", () => void addAccount().catch(showAuthError));
    $("switch-account").addEventListener("click", () => void switchAccount().catch(showAuthError));
    $("save-config").addEventListener("click", () => void saveConfig().catch(showConfigError));
    $("load-models").addEventListener("click", () => void loadModels().catch(showConfigError));
    form.elements.namedItem("runtime.proxy_enabled").addEventListener("change", (event) => {
        void updateService(event.currentTarget.checked).catch(showConfigError);
    });
    $("language-select").addEventListener("change", (event) => {
        applyI18n(event.currentTarget.value);
    });
    $("theme-select").addEventListener("change", (event) => {
        applyTheme(event.currentTarget.value);
    });
}
async function refreshAll() {
    await Promise.all([loadStatus(), loadStats(), loadQuota()]);
}
function showAuthError(error) {
    setMessage("auth-message", error instanceof Error ? error.message : String(error), true);
}
function showConfigError(error) {
    setMessage("config-message", error instanceof Error ? error.message : String(error), true);
}
wire();
void Promise.all([loadConfig(), loadStatus(), loadStats(), loadQuota()])
    .then(() => loadModels().catch(() => undefined))
    .catch((error) => {
    showAuthError(error);
    showConfigError(error);
});
window.setInterval(() => void loadStats().catch(() => undefined), 5000);
