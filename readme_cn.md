[English](./readme.md) | [中文](./readme_cn.md)

# Copilot Proxy

将 GitHub Copilot 转为 OpenAI 兼容 API。后端使用 Go，长期 GitHub token 存入系统 keyring，配置集中放在 JSON 文件中，内置 WebUI 使用 TypeScript。

## 特性

- OpenAI 兼容代理：继续使用 `http://localhost:15432/v1/chat/completions` 等路径。
- Anthropic Messages API 兼容：`http://localhost:15432/v1/messages`。
- Gemini API 兼容：`http://localhost:15432/v1beta/models/{model}:generateContent`。
- 保持 CLI 习惯：直接运行 `copilot-proxy` 原生二进制即可启动。
- 系统 keyring：Windows Credential Manager、macOS Keychain、Linux Secret Service，不再明文保存 `.copilot_token.json`。
- 统一配置文件：默认位于系统配置目录，也支持 `--config path` 或 `COPILOT_PROXY_CONFIG`。
- TypeScript 前端：启动后访问 `http://localhost:15432/ui/` 查看状态、授权、修改配置。

## 构建

```bash
cd frontend
npm install
npm run build
cd ..
go build -o copilot-proxy ./cmd/copilot-proxy
```

在 Windows 上，Go 会自动生成 `copilot-proxy.exe`，也可以显式指定：

```bash
go build -o copilot-proxy.exe ./cmd/copilot-proxy
```

前端构建脚本 `frontend/build.js` 使用 Node.js API，**Windows、macOS、Linux 全平台兼容**，不再依赖 Unix shell 命令。

也可以一条命令完成前端 + Go 编译：

```bash
cd frontend && npm run build:all
```

**注意：** Go 会把 `internal/web/dist` 打进二进制（`go:embed`）。只执行 `npm run build` 再重启旧的可执行文件，页面不会变。任选其一即可看到最新 UI：

1. 在项目根目录运行服务（会自动从磁盘加载 `internal/web/dist`，无需重编 Go）
2. 修改前端后执行 `go build` 或 `npm run build:all` 再重启
3. 设置环境变量 `COPILOT_PROXY_UI_DIST=/path/to/dist` 指向构建产物目录

Linux 需要可用的 Secret Service keyring（常见桌面环境已提供）。无桌面环境的服务器通常需要安装并启动 `gnome-keyring` 或兼容实现。

## 使用

启动代理：

```bash
# macOS / Linux
./copilot-proxy

# Windows
copilot-proxy.exe
```

然后打开 WebUI：

```text
http://localhost:15432/ui/
```

如果你想跳过终端授权，直接从 WebUI 完成授权，可以这样启动：

```bash
# macOS / Linux
./copilot-proxy --no-login serve

# Windows
copilot-proxy.exe --no-login serve
```

首次运行会打开 GitHub 设备授权页面，按终端提示输入验证码。授权完成后，GitHub token 写入系统 keyring，Copilot 短期 token 只保存在进程内并定时刷新。

常用命令：

```bash
# macOS / Linux
./copilot-proxy login
./copilot-proxy logout
./copilot-proxy config show
./copilot-proxy config path
./copilot-proxy --config ./config.example.json serve

# Windows
copilot-proxy.exe login
copilot-proxy.exe logout
copilot-proxy.exe config show
copilot-proxy.exe config path
copilot-proxy.exe --config ./config.example.json serve
```

## 配置

默认配置首次运行时自动创建。也可以复制 `config.example.json` 后通过 `--config` 指定。

关键字段：

- `server.host` / `server.port`：监听地址和端口。
- `copilot.api_base`：GitHub Copilot API 地址。
- `security.api_key`：本地兼容 API 使用的访问密钥，可在 WebUI 或配置文件中修改。
- `auth.accounts` / `auth.active_account_id`：多 GitHub 账号列表和当前使用的账号。
- `keyring.service` / `keyring.account`：旧版兼容字段；WebUI 的高级选项中也可以修改当前账号的 keyring 条目。
- `fallback.preferred_prefixes`：模型不可用时的回退优先级；WebUI 会优先通过 Copilot 模型列表生成可勾选的图形化选择器。
- `runtime.proxy_disabled`：服务开关，打开后兼容 API 暂停响应请求。
- `ui.language` / `ui.theme`：WebUI 语言和颜色主题，支持中文/英文、浅色/深色/跟随系统。
- `frontend.enabled`：是否启用 `/ui/`。

修改端口示例：

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 15432
  }
}
```

## Continue 配置

```yaml
models:
  - name: GPT-5.4 (Copilot Proxy)
    provider: openai
    model: gpt-5.4
    apiBase: http://localhost:15432
    apiKey: dummy
    roles:
      - chat
      - edit
```

## 兼容 API

OpenAI:

```bash
curl http://localhost:15432/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer dummy" \
  -d '{"model":"gpt-4.1","messages":[{"role":"user","content":"hello"}]}'
```

Anthropic:

```bash
curl http://localhost:15432/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: dummy" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model":"claude-sonnet-4.6","max_tokens":1024,"messages":[{"role":"user","content":"hello"}]}'
```

Gemini:

```bash
curl "http://localhost:15432/v1beta/models/gemini-pro:generateContent?key=dummy" \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}'
```

## API

- `GET /`：健康检查。
- `GET /fallback`：当前 fallback 模型。
- `POST /v1/messages`：Anthropic Messages API 兼容接口。
- `POST /v1beta/models/{model}:generateContent`：Gemini generateContent 兼容接口。
- `POST /v1beta/models/{model}:streamGenerateContent`：Gemini 流式兼容接口。
- `GET /api/status`：GUI 状态。
- `GET /api/config`：读取配置。
- `PUT /api/config`：保存配置并尽量立即应用。
- `GET /api/models`：读取当前 Copilot 账号可见的模型列表，用于 WebUI 图形化选择回退优先级。
- `GET /api/stats`：读取请求数量、成功/失败、token 用量和最近请求记录。
- `POST /api/service`：启用或暂停兼容 API 服务。
- `GET /api/accounts` / `POST /api/accounts`：读取或新增 GitHub 账号配置。
- `POST /api/accounts/switch`：切换当前请求使用的 GitHub 账号。
- `GET /api/quota`：尝试读取 GitHub Copilot 额度信息；如果当前账号或接口不返回稳定数据，WebUI 会显示不可用。
- `POST /api/auth/device/start`：开始设备授权。
- `POST /api/auth/device/poll`：轮询授权结果。
- `POST /api/auth/logout`：删除 keyring 中的 GitHub token。

## 安全说明

长期 GitHub token 不再写入项目目录或 JSON 文件。旧版 `.copilot_token.json` 不会再被 Go 后端读取；确认不需要回滚后可以删除它。

## License

MIT
