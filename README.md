[English](./README.md) | [中文](./README_zh-CN.md)

# Copilot Proxy

Expose GitHub Copilot as an OpenAI, Anthropic, and Gemini compatible API
Built with Go + Vue 3, single binary with embedded WebUI, zero external runtime dependencies

## Features

- OpenAI-compatible proxy endpoint at `http://localhost:15432/v1/chat/completions`
- Anthropic Messages API compatibility at `http://localhost:15432/v1/messages`
- Gemini API compatibility at `http://localhost:15432/v1beta/models/{model}:generateContent`
- Single binary — Go backend with embedded Vue 3 + Naive UI frontend via `go:embed`
- AES-GCM encrypted token storage using machine-specific key derivation, no system keyring required
- Multi-account support — add and switch between multiple GitHub accounts via WebUI
- Configurable model fallback with graphical picker in WebUI
- Request statistics and real-time quota display
- Streaming support with proper SSE handling
- Cross-platform: Windows, macOS, Linux (including headless servers)

## Quick Start

```
git clone https://github.com/Open-Copilot-Proxy/Copilot_Proxy.git
cd Copilot_Proxy

# Build frontend
cd web && npm install && npm run build && cd ..

# Build Go binary
go build -o copilot-proxy ./cmd/copilot-proxy

# Run
./copilot-proxy
```

Or use the Makefile:

```
make build    # builds frontend + Go binary in one step
```

On Windows:

```
go build -o copilot-proxy.exe ./cmd/copilot-proxy
.\copilot-proxy.exe
```

Then open **http://localhost:15432/ui/** in your browser

## Usage

### Start the server

```bash
./copilot-proxy serve

# Or just run without arguments
./copilot-proxy
```

The server starts immediately without blocking on authorization
If a saved token exists, it refreshes automatically
If no token, the WebUI shows a login page — complete GitHub authorization from the browser

### Command-line reference

```bash
copilot-proxy                  # Alias for `serve`
copilot-proxy serve            # Start proxy server
copilot-proxy login            # Run GitHub device authorization in terminal
copilot-proxy logout           # Remove current account
copilot-proxy config show      # Print effective configuration
copilot-proxy config path      # Print config file location
copilot-proxy --config <path>  # Use custom config file
```

### First-time setup

1. Start the server: `./copilot-proxy`
2. Open the WebUI at `http://localhost:15432/ui/`
3. In the WebUI, click **添加 GitHub 账号** to start device authorization
4. Copy the verification code and open the provided GitHub URL
5. After authorization, the account appears and is ready to use
6. Optionally set an **管理密码** (admin password) in config to protect the WebUI

### Headless Linux

No desktop environment required
Start the server, check the logs for the WebUI URL, and access it from another machine on the same network
All authorization can be done through the WebUI — no terminal interaction needed

## Build

### Prerequisites

- Go 1.23+
- Node.js 20+

### Build steps

```bash
# 1. Install frontend dependencies and build
cd web
npm install
npm run build
cd ..

# 2. Copy frontend to embed directory
cp -r web/dist/* internal/web/dist/

# 3. Build Go binary (embeds frontend automatically)
go build -ldflags="-s -w" -o copilot-proxy ./cmd/copilot-proxy
```

### Cross-compilation

```bash
make build-all
```

This produces binaries for Windows (amd64), Linux (amd64 + arm64), and macOS (amd64 + arm64)

### Docker

```bash
make docker
# or
docker build -t copilot-proxy .
```

## Configuration

The default config is auto-created at `<exe_dir>/config/config.json` on first run
You can override with `--config <path>` or the `COPILOT_PROXY_CONFIG` environment variable

A template is available at `config.example.json`

### Key fields

- `server.host` / `server.port` — bind address and port (default: `0.0.0.0:15432`)
- `security.admin_password` — password to protect the WebUI. If empty, WebUI has no login
- `security.api_key` — optional API key for compatible API requests
- `copilot.api_base` — GitHub Copilot API base URL
- `auth.accounts` — list of GitHub accounts (token is stored encrypted, not in plain text)
- `auth.active_account_id` — currently active account
- `fallback.preferred_prefixes` — ordered list of model prefixes for fallback selection
- `runtime.proxy_disabled` — toggle to pause compatible API requests
- `ui.language` — `zh` or `en`
- `ui.theme` — `system`, `light`, or `dark`

### WebUI access control

By default, the WebUI is protected with password `admin` (set in `security.admin_password`)
Change it to a secure password in `config.json` or via the WebUI settings page
The password is hashed and stored in the config file
If no password is set, the WebUI is open to anyone on the local network — set this for production use

> **Security note**: The default password `admin` is provided for convenience.
> Change it immediately if the proxy is exposed to untrusted networks.

## Compatible APIs

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

## API Reference

- `GET /` — health check
- `GET /fallback` — current fallback model info
- `POST /v1/chat/completions` — OpenAI chat completions
- `POST /v1/messages` — Anthropic Messages API
- `POST /v1beta/models/{model}:generateContent` — Gemini generateContent
- `POST /v1beta/models/{model}:streamGenerateContent` — Gemini streaming
- `GET /api/status` — service and token status
- `GET /api/config` — read current config
- `PUT /api/config` — save config (applied without restart)
- `GET /api/models` — list models visible to active Copilot account
- `GET /api/stats` — request statistics (counts, tokens, recent records)
- `POST /api/service` — enable or pause proxy service
- `GET /api/accounts` — list GitHub accounts
- `POST /api/accounts` — add a GitHub account
- `POST /api/accounts/switch` — switch active account
- `DELETE /api/accounts/:id` — remove an account
- `GET /api/quota` — probe GitHub Copilot quota
- `POST /api/auth/login` — WebUI admin login
- `POST /api/auth/device/start` — start device authorization flow
- `POST /api/auth/device/poll` — poll device authorization status
- `POST /api/auth/logout` — remove current account (legacy)

## Token Security

GitHub tokens are encrypted at rest using **AES-256-GCM** before being written to `config.json`
The encryption key is derived from the machine's hardware fingerprint:

- **Windows**: `MachineGuid` from registry
- **Linux**: `/etc/machine-id`
- **macOS**: `IOPlatformUUID` from I/O Kit

This means the config file is tied to a specific machine and cannot be decrypted on another device
No system keyring, no third-party credential managers, no plaintext token files

## Architecture

```
cmd/copilot-proxy/          # Entry point
internal/
├── config/                 # Config load/save, AES-GCM crypto, machine fingerprint
├── auth/                   # Account manager, GitHub device OAuth flow
├── proxy/
│   ├── proxy.go            # OpenAI-compatible request forwarding
│   ├── anthropic.go        # Anthropic protocol conversion
│   ├── gemini.go           # Gemini protocol conversion
│   ├── fallback.go         # Fallback model selection
│   └── stats.go            # Request statistics (rolling window)
├── server/                 # HTTP server, routing, SSE handling, admin auth
└── web/
    ├── assets.go            # go:embed embedded frontend
    └── middleware.go        # WebUI login middleware
web/                         # Vue 3 + TypeScript + Naive UI frontend
```

## License

MIT
