[English](./readme.md) | [中文](./readme_cn.md)

# Copilot Proxy

Expose GitHub Copilot as an OpenAI-compatible API. The backend has been rewritten in Go, the long-lived GitHub token is stored in the system keyring, configuration is centralized in JSON, and a TypeScript UI is available at runtime.

## Features

- OpenAI-compatible proxy endpoints such as `http://localhost:15432/v1/chat/completions`.
- CLI-compatible startup: run the native binary directly; `python main.py` delegates to the Go backend for old workflows.
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

Legacy launcher:

```bash
python main.py
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
- `keyring.service` / `keyring.account`: system keyring item names.
- `fallback.preferred_prefixes`: fallback model priority.
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

## API

- `GET /`: health check.
- `GET /fallback`: selected fallback model.
- `GET /api/status`: UI status.
- `GET /api/config`: read config.
- `PUT /api/config`: save config, applied after restart.
- `POST /api/auth/device/start`: start device auth.
- `POST /api/auth/device/poll`: poll device auth.
- `POST /api/auth/logout`: delete the GitHub token from keyring.

## Security

The Go backend does not read or write the old `.copilot_token.json` plaintext token file. After confirming you do not need rollback compatibility, remove that file.

## License

MIT
