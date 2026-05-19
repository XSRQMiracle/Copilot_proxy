[English](./README.md) | [中文](./README_zh-CN.md)

# Copilot Proxy

将 GitHub Copilot 转为 OpenAI、Anthropic 和 Gemini 兼容 API
Go + Vue 3 实现，单二进制嵌入 WebUI，零外部运行时依赖

## 特性

- OpenAI 兼容端点：`http://localhost:15432/v1/chat/completions`
- Anthropic Messages API 兼容：`http://localhost:15432/v1/messages`
- Gemini API 兼容：`http://localhost:15432/v1beta/models/{model}:generateContent`
- 单文件二进制 —— Go 后端通过 `go:embed` 嵌入 Vue 3 + Naive UI 前端
- Token 使用 AES-256-GCM 加密存储，密钥从机器硬件指纹派生，无需系统 keyring
- 多账号支持：WebUI 内添加和切换多个 GitHub 账号
- 请求统计和实时额度展示
- 完善的 SSE 流式响应支持
- 跨平台：Windows、macOS、Linux（含无头服务器）

## 快速开始

```
git clone https://github.com/Open-Copilot-Proxy/Copilot_Proxy.git
cd Copilot_Proxy

# 构建前端
cd web && npm install && npm run build && cd ..

# 编译 Go 二进制
go build -o copilot-proxy ./cmd/copilot-proxy

# 运行
./copilot-proxy
```

或用 Makefile 一步完成：

```
make build
```

macOS / Linux：

```
go build -o copilot-proxy ./cmd/copilot-proxy
chmod +x ./copilot-proxy
./copilot-proxy
```

Windows 上：

```
go build -o copilot-proxy.exe ./cmd/copilot-proxy
.\copilot-proxy.exe
```

然后打开浏览器访问 **http://localhost:15432/ui/**

## 使用方法

### 启动服务

```bash
./copilot-proxy serve

# 或不加参数（同上）
./copilot-proxy
```

服务启动后不会阻塞等待授权
如果已保存 Token，会自动刷新
如果没有 Token，WebUI 会显示登录页，在浏览器中完成 GitHub 授权即可

### 命令行参考

```bash
copilot-proxy                  # 等同于 `serve`
copilot-proxy serve            # 启动代理服务
copilot-proxy login            # 终端内执行 GitHub 设备授权
copilot-proxy logout           # 移除当前账号
copilot-proxy config show      # 打印当前配置
copilot-proxy config path      # 打印配置文件路径
copilot-proxy --config <path>  # 使用自定义配置文件
```

### 首次使用

1. 启动服务：`./copilot-proxy`
2. 打开 WebUI：`http://localhost:15432/ui/`
3. 点击 **添加 GitHub 账号** 开始设备授权
4. 复制验证码并打开 GitHub 授权页面
5. 授权完成后账号自动生效
6. 如需保护 WebUI，在配置中设置 **管理密码**

> **macOS 和 Linux 用户**：如果从 Releases 下载预编译二进制，可能需要先赋予可执行权限：
> ```bash
> chmod +x ./copilot-proxy
> ```
> 如果 macOS 提示"无法打开，未验证开发者"，请前往**系统设置 > 隐私与安全性**，点击**仍要打开**。

### 无头 Linux

不需要桌面环境
启动服务后，查看日志中的 WebUI 地址，从同网络的另一台机器访问即可
所有授权操作都可在 WebUI 中完成，无需终端交互

## 构建

### 前置要求

- Go 1.23+
- Node.js 20+

### 构建步骤

```bash
# 1. 安装前端依赖并构建
cd web
npm install
npm run build
cd ..

# 2. 将前端产物复制到 Go embed 目录
cp -r web/dist/* internal/web/dist/

# 3. 编译 Go 二进制（自动嵌入前端）
go build -ldflags="-s -w" -o copilot-proxy ./cmd/copilot-proxy
```

### 交叉编译

```bash
make build-all
```

生成 Windows (amd64)、Linux (amd64 + arm64)、macOS (amd64 + arm64) 各平台二进制

### Docker

```bash
make docker
# 或
docker build -t copilot-proxy .
```

## 配置

首次运行自动在 `<exe_dir>/config/config.json` 生成默认配置
可通过 `--config <path>` 或 `COPILOT_PROXY_CONFIG` 环境变量覆盖

模板文件见 `config.example.json`

### 关键字段

- `server.host` / `server.port` — 监听地址和端口（默认 `0.0.0.0:15432`）
- `security.admin_password` — WebUI 管理密码。为空则无需登录
- `security.api_key` — 兼容 API 的可选访问密钥
- `copilot.api_base` — GitHub Copilot API 地址
- `auth.accounts` — GitHub 账号列表（Token 加密存储，非明文）
- `auth.active_account_id` — 当前使用的账号 ID
- `runtime.proxy_disabled` — 暂停兼容 API 服务的开关
- `ui.language` — `zh` 或 `en`
- `ui.theme` — `system`、`light` 或 `dark`

### WebUI 访问控制

设置 `security.admin_password` 后，WebUI 需要登录才能访问
密码明文存储在配置文件中
若未设置密码，局域网内可任意访问 WebUI —— 生产环境建议配置密码

## 兼容 API

### OpenAI

```bash
curl http://localhost:15432/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer dummy" \
  -d '{"model":"gpt-4.1","messages":[{"role":"user","content":"hello"}]}'
```

### Anthropic

```bash
curl http://localhost:15432/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: dummy" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model":"claude-sonnet-4.6","max_tokens":1024,"messages":[{"role":"user","content":"hello"}]}'
```

### Gemini

```bash
curl "http://localhost:15432/v1beta/models/gemini-pro:generateContent?key=dummy" \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}'
```

## API 参考

- `GET /` — 健康检查
- `GET /v1/models` — 列出可用模型
- `POST /v1/chat/completions` — OpenAI 聊天补全
- `POST /v1/messages` — Anthropic Messages API
- `POST /v1beta/models/{model}:generateContent` — Gemini generateContent
- `POST /v1beta/models/{model}:streamGenerateContent` — Gemini 流式接口
- `GET /api/status` — 服务和 Token 状态
- `GET /api/config` — 读取配置
- `PUT /api/config` — 保存配置（不需重启立即生效）
- `GET /api/models` — 查看 Copilot 账号下的可用模型
- `GET /api/stats` — 请求统计（数量、Token、最近记录）
- `POST /api/service` — 启用或暂停代理服务
- `GET /api/accounts` — 列出 GitHub 账号
- `POST /api/accounts` — 添加 GitHub 账号
- `POST /api/accounts/switch` — 切换当前账号
- `DELETE /api/accounts/:id` — 删除账号
- `GET /api/quota` — 探测 GitHub Copilot 额度
- `POST /api/auth/login` — WebUI 管理登录
- `POST /api/auth/device/start` — 开始设备授权
- `POST /api/auth/device/poll` — 轮询授权状态
- `POST /api/auth/logout` — 移除当前账号（旧版）

## Token 安全

GitHub Token 在写入 `config.json` 前会使用 **AES-256-GCM** 加密
加密密钥从机器硬件指纹派生：

- **Windows**：注册表中的 `MachineGuid`
- **Linux**：`/etc/machine-id`
- **macOS**：I/O Kit 的 `IOPlatformUUID`

这意味着配置文件与特定机器绑定，无法在其他设备上解密
无需系统 keyring、无需第三方凭据管理器、无明文 Token 文件

## 项目结构

```
cmd/copilot-proxy/          # 入口（CLI 子命令、信号处理、服务器启动）
internal/
├── config/
│   ├── config.go            # 配置加载/保存/验证（默认值、热重载）
│   ├── config_test.go       # 配置测试
│   └── crypto.go            # AES-GCM 加密/解密（机器指纹派生密钥）
├── auth/
│   ├── auth.go              # 账号管理器（多账号增删改查 + Token 生命周期）
│   └── oauth.go             # GitHub Device Code Flow 客户端
├── proxy/
│   ├── proxy.go             # OpenAI 兼容请求转发 + 主处理器
│   ├── anthropic.go         # Anthropic Messages API 协议转换
│   ├── gemini.go            # Gemini API 协议转换
│   ├── compat.go            # 协议兼容工具函数
│   └── stats.go             # 请求统计（环形缓冲区）
├── server/                  # HTTP 服务器、路由、SSE、管理认证、WebUI API
└── web/
    ├── assets.go            # go:embed 前端静态资源 dist
    ├── favicon.go           # Favicon 图标数据 (go:embed)
    ├── favicon.png          # Favicon 图标文件
    └── fs.go                # 前端静态文件服务（磁盘优先，回退 embed）
web/                         # Vue 3 + TypeScript + Naive UI 前端
```

## License

MIT
