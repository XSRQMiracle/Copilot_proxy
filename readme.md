[English](./readme.md) | [中文](./readme_cn.md)

# Copilot Proxy

Expose GitHub Copilot as an OpenAI-compatible API. The backend is written in Go, the long-lived GitHub token is stored in the system keyring, configuration is centralized in JSON, and the built-in WebUI is written in TypeScript.

## Features

- OpenAI-compatible proxy endpoints such as `http://localhost:15432/v1/chat/completions`.
- Anthropic Messages API compatibility at `http://localhost:15432/v1/messages`.
- Gemini API compatibility at `http://localhost:15432/v1beta/models/{model}:generateContent`.
- Native CLI startup: run the `copilot-proxy` binary directly.
- System keyring storage: Windows Credential Manager, macOS Keychain, and Linux Secret Service.
- Central JSON config via the default OS config directory, `--config path`, or `COPILOT_PROXY_CONFIG`.
- TypeScript UI at `http://localhost:15432/ui/` for status, login, logout, and config editing.

## Build

```bash
cd frontend
npm install
npm run build
cd ..
go build -o copilot-proxy ./cmd/copilot-proxy
```

Linux requires a working Secret Service keyring. Desktop environments usually provide one; headless servers may need `gnome-keyring` or a compatible service.

## Usage

Start the proxy:

```bash
./copilot-proxy
```

Then open the WebUI:

```text
http://localhost:15432/ui/
```

If you want to skip terminal login and complete authorization from the WebUI, start the server with:

```bash
./copilot-proxy --no-login serve
```

Common commands:

```bash
./copilot-proxy login
./copilot-proxy logout
./copilot-proxy config show
./copilot-proxy config path
./copilot-proxy --config ./config.example.json serve
```

On first run the app starts the GitHub device authorization flow. The GitHub token is saved to the system keyring; the short-lived Copilot token stays in memory and refreshes automatically.

## Configuration

The default config is created on first run. You can also copy `config.example.json` and pass it with `--config`.

Important fields:

- `server.host` / `server.port`: bind address and port.
- `copilot.api_base`: GitHub Copilot API base URL.
- `security.api_key`: local compatible API key, editable from the config file or WebUI.
- `auth.accounts` / `auth.active_account_id`: GitHub account list and the active request account.
- `keyring.service` / `keyring.account`: legacy compatibility fields; the active account keyring item is also editable in the WebUI advanced section.
- `fallback.preferred_prefixes`: fallback model priority. The WebUI first tries to load Copilot's visible model list and exposes it as a graphical picker.
- `runtime.proxy_disabled`: service switch for pausing compatible API requests.
- `ui.language` / `ui.theme`: WebUI language and theme, supporting Chinese/English and system/light/dark modes.
- `frontend.enabled`: enables `/ui/`.

## Continue Example

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

## Compatible APIs

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

- `GET /`: health check.
- `GET /fallback`: selected fallback model.
- `POST /v1/messages`: Anthropic Messages API compatible endpoint.
- `POST /v1beta/models/{model}:generateContent`: Gemini generateContent compatible endpoint.
- `POST /v1beta/models/{model}:streamGenerateContent`: Gemini streaming compatible endpoint.
- `GET /api/status`: UI status.
- `GET /api/config`: read config.
- `PUT /api/config`: save config and apply it where possible without restart.
- `GET /api/models`: read models visible to the active Copilot account for graphical fallback selection.
- `GET /api/stats`: read request counts, success/failure totals, token usage, and recent request records.
- `POST /api/service`: enable or pause compatible API requests.
- `GET /api/accounts` / `POST /api/accounts`: read or add GitHub account entries.
- `POST /api/accounts/switch`: switch the active GitHub account used for requests.
- `GET /api/quota`: best-effort GitHub Copilot quota probe. If the active account/API does not return stable quota data, the WebUI reports it as unavailable.
- `POST /api/auth/device/start`: start device auth.
- `POST /api/auth/device/poll`: poll device auth.
- `POST /api/auth/logout`: delete the GitHub token from keyring.

## Security

The Go backend does not read or write the old `.copilot_token.json` plaintext token file. After confirming you do not need rollback compatibility, remove that file.

## License

MIT
