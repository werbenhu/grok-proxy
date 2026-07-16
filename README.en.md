# GrokProxy

English | [简体中文](README.md)

Connect Grok / xAI and expose OpenAI Chat Completions and Anthropic Messages compatible endpoints on your local machine.

Listens on `127.0.0.1:8181` by default; closing the app stops the proxy.

## Features

- **xAI API Key**: connect directly to `api.x.ai`.
- **Grok device authorization**: official xAI OAuth Device Flow with automatic token refresh before expiry.
- `GET /v1/models`
- `POST /v1/chat/completions`: OpenAI JSON / SSE, image input, function tools, and reasoning fields.
- `POST /v1/messages`: Anthropic JSON / SSE, system, images, tool calls, and thinking.
- Optional single "local proxy key" — no account or permission model introduced.

## Download

Get the desktop app for your platform from [Releases](../../releases):

| Platform | File |
| --- | --- |
| Windows x64 | `GrokProxy-*-windows-amd64.exe` |
| Windows ARM64 | `GrokProxy-*-windows-arm64.exe` |
| macOS Intel | `GrokProxy-*-darwin-amd64.app.zip` |
| macOS Apple Silicon | `GrokProxy-*-darwin-arm64.app.zip` |
| Linux x64 | `GrokProxy-*-linux-amd64.tar.gz` |
| Linux ARM64 | `GrokProxy-*-linux-arm64.tar.gz` |

On macOS, if Gatekeeper blocks the first launch, right-click the app in Finder and choose **Open**, or allow it under **System Settings → Privacy & Security**.

## Usage

1. Open GrokProxy.
2. Use **Site auth** to sign in to Grok, or switch to **xAI API Key** and paste a key.
3. Keep the default listen address and start the proxy.
4. Point your client's Base URL at the local address shown in the UI.

### OpenAI compatible

```bash
export OPENAI_BASE_URL="http://127.0.0.1:8181/v1"
export OPENAI_API_KEY="not-needed"
export OPENAI_MODEL="grok-4.5"
```

```bash
curl http://127.0.0.1:8181/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer not-needed" \
  -d '{"model":"grok-4.5","messages":[{"role":"user","content":"Hello"}]}'
```

### Anthropic compatible

```bash
export ANTHROPIC_BASE_URL="http://127.0.0.1:8181"
export ANTHROPIC_API_KEY="not-needed"
export ANTHROPIC_MODEL="grok-4.5"
```

```bash
curl http://127.0.0.1:8181/v1/messages \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -H "x-api-key: not-needed" \
  -d '{"model":"grok-4.5","max_tokens":512,"messages":[{"role":"user","content":"Hello"}]}'
```

If you set a local proxy key, replace `not-needed` with that key. OpenAI requests use `Authorization: Bearer <key>`; Anthropic requests may use `x-api-key: <key>`.

## Configuration & security

Configuration is stored in the user config directory:

- Windows: `%AppData%\GrokProxy\config.json`
- macOS: `~/Library/Application Support/GrokProxy/config.json`
- Linux: `$XDG_CONFIG_HOME/GrokProxy/config.json` or `~/.config/GrokProxy/config.json`

With the default loopback listener, no local proxy key is required; when listening on `0.0.0.0`, a LAN IP, or any non-loopback address, the app requires a local proxy key.
