"use strict";
const $ = (id) => {
    const element = document.getElementById(id);
    if (!element)
        throw new Error(`Missing element #${id}`);
    return element;
};
const form = $("config-form");
let currentConfig = null;
let pollTimer;
async function request(url, init) {
    const response = await fetch(url, {
        headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
        ...init,
    });
    if (!response.ok && response.status !== 202) {
        const payload = await response.json().catch(() => ({}));
        throw new Error(payload.error ?? `${response.status} ${response.statusText}`);
    }
    return response.json();
}
async function loadStatus() {
    const status = await request("/api/status");
    $("base-url").textContent = status.base_url;
    $("config-path").textContent = status.config_path;
    $("fallback-model").textContent = status.fallback_model || "-";
    $("expires-at").textContent = status.copilot_expires_at ?? "-";
    setPill("github-state", "GitHub", status.github_token_ready);
    setPill("copilot-state", "Copilot", status.copilot_token_ready);
}
async function loadConfig() {
    currentConfig = await request("/api/config");
    setField("server.host", currentConfig.server.host);
    setField("server.port", String(currentConfig.server.port));
    setField("copilot.api_base", currentConfig.copilot.api_base);
    setField("copilot.integration_id", currentConfig.copilot.integration_id);
    setField("keyring.service", currentConfig.keyring.service);
    setField("keyring.account", currentConfig.keyring.account);
    setField("fallback.preferred_prefixes", currentConfig.fallback.preferred_prefixes.join(", "));
}
async function saveConfig() {
    if (!currentConfig)
        return;
    const next = structuredClone(currentConfig);
    next.server.host = getField("server.host");
    next.server.port = Number(getField("server.port"));
    next.copilot.api_base = getField("copilot.api_base");
    next.copilot.integration_id = getField("copilot.integration_id");
    next.keyring.service = getField("keyring.service");
    next.keyring.account = getField("keyring.account");
    next.fallback.preferred_prefixes = getField("fallback.preferred_prefixes")
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean);
    await request("/api/config", { method: "PUT", body: JSON.stringify(next) });
    currentConfig = next;
    setMessage("config-message", "配置已保存，重启代理后生效");
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
    setMessage("auth-message", "等待 GitHub 授权");
    window.clearInterval(pollTimer);
    pollTimer = window.setInterval(() => void pollAuth(), Math.max(flow.interval, 5) * 1000);
}
async function pollAuth() {
    const result = await request("/api/auth/device/poll", { method: "POST", body: "{}" });
    if (result.status === "authorized") {
        window.clearInterval(pollTimer);
        $("device-box").classList.add("hidden");
        setMessage("auth-message", "授权完成");
        await loadStatus();
    }
    else {
        setMessage("auth-message", `等待中：${result.status}`);
    }
}
async function logout() {
    await request("/api/auth/logout", { method: "POST", body: "{}" });
    setMessage("auth-message", "已从系统 keyring 删除 token");
    await loadStatus();
}
function setPill(id, label, ready) {
    const element = $(id);
    element.textContent = `${label}: ${ready ? "ready" : "missing"}`;
    element.classList.toggle("ok", ready);
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
function setMessage(id, message, error = false) {
    const element = $(id);
    element.textContent = message;
    element.classList.toggle("error", error);
}
function wire() {
    $("refresh").addEventListener("click", () => void loadStatus().catch(showAuthError));
    $("start-auth").addEventListener("click", () => void startAuth().catch(showAuthError));
    $("logout").addEventListener("click", () => void logout().catch(showAuthError));
    $("save-config").addEventListener("click", () => void saveConfig().catch(showConfigError));
}
function showAuthError(error) {
    setMessage("auth-message", error instanceof Error ? error.message : String(error), true);
}
function showConfigError(error) {
    setMessage("config-message", error instanceof Error ? error.message : String(error), true);
}
wire();
void Promise.all([loadStatus(), loadConfig()]).catch((error) => {
    showAuthError(error);
    showConfigError(error);
});
