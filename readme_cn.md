[English](./readme.md) | [中文](./readme_cn.md)

# Copilot Proxy

将 GitHub Copilot 转为 OpenAI 兼容 API。后端已重写为 Go，长期 GitHub token 存入系统 keyring，配置集中放在 JSON 文件中，并提供 TypeScript 图形界面。

## 特性

- OpenAI 兼容代理：继续使用 `http://localhost:15432/v1/chat/completions` 等路径。
- 保持 CLI 习惯：直接运行二进制即可启动；`python main.py` 会转发到 Go 版本。
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

Linux 需要可用的 Secret Service keyring（常见桌面环境已提供）。无桌面环境的服务器通常需要安装并启动 `gnome-keyring` 或兼容实现。

## 使用

启动代理：

```bash
./copilot-proxy
```

兼容旧入口：

```bash
python main.py
```

首次运行会打开 GitHub 设备授权页面，按终端提示输入验证码。授权完成后，GitHub token 写入系统 keyring，Copilot 短期 token 只保存在进程内并定时刷新。

常用命令：

```bash
./copilot-proxy login
./copilot-proxy logout
./copilot-proxy config show
./copilot-proxy config path
./copilot-proxy --config ./config.example.json serve
```

## 配置

默认配置首次运行时自动创建。也可以复制 `config.example.json` 后通过 `--config` 指定。

关键字段：

- `server.host` / `server.port`：监听地址和端口。
- `copilot.api_base`：GitHub Copilot API 地址。
- `keyring.service` / `keyring.account`：系统 keyring 中的条目名称。
- `fallback.preferred_prefixes`：模型不可用时的回退优先级。
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

## API

- `GET /`：健康检查。
- `GET /fallback`：当前 fallback 模型。
- `GET /api/status`：GUI 状态。
- `GET /api/config`：读取配置。
- `PUT /api/config`：保存配置，重启后生效。
- `POST /api/auth/device/start`：开始设备授权。
- `POST /api/auth/device/poll`：轮询授权结果。
- `POST /api/auth/logout`：删除 keyring 中的 GitHub token。

## 安全说明

长期 GitHub token 不再写入项目目录或 JSON 文件。旧版 `.copilot_token.json` 不会再被 Go 后端读取；确认不需要回滚后可以删除它。

## License

MIT
