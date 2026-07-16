# GrokProxy

[English](README.en.md) | 简体中文

GrokProxy — 在本地为 Grok/xAI 提供 OpenAI 与 Anthropic 兼容 API。

默认监听 `127.0.0.1:8181`，关闭程序即停止代理。

## 能力

- **xAI API Key**：直接连接 `api.x.ai`。
- **Grok 设备授权**：使用 xAI 官方 OAuth Device Flow，令牌到期前自动刷新。
- `GET /v1/models`
- `POST /v1/chat/completions`：OpenAI JSON / SSE、图片输入、函数工具与推理字段。
- `POST /v1/messages`：Anthropic JSON / SSE、system、图片、工具调用与 thinking。
- 可选的单一“本地代理密钥”，不引入账号和权限模型。

## 下载

从 [Releases](../../releases) 下载对应平台的桌面程序：

| 平台 | 文件 |
| --- | --- |
| Windows x64 | `GrokProxy-*-windows-amd64.exe` |
| Windows ARM64 | `GrokProxy-*-windows-arm64.exe` |
| macOS Intel | `GrokProxy-*-darwin-amd64.app.zip` |
| macOS Apple Silicon | `GrokProxy-*-darwin-arm64.app.zip` |
| Linux x64 | `GrokProxy-*-linux-amd64.tar.gz` |
| Linux ARM64 | `GrokProxy-*-linux-arm64.tar.gz` |

macOS 首次打开若被拦截，可在访达中右键应用选择「打开」，或在「系统设置 → 隐私与安全性」中允许。

## 使用

1. 打开 GrokProxy。
2. 使用「网站授权」登录 Grok，或切换到「xAI API Key」填写密钥。
3. 保持默认监听地址并启动代理。
4. 把客户端的 Base URL 指向界面展示的本地地址。

### OpenAI 兼容

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

### Anthropic 兼容

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

如果设置了本地代理密钥，把示例中的 `not-needed` 换成该密钥。OpenAI 请求使用 `Authorization: Bearer <key>`，Anthropic 请求可使用 `x-api-key: <key>`。

## 配置与安全

配置保存在系统用户配置目录：

- Windows：`%AppData%\GrokProxy\config.json`
- macOS：`~/Library/Application Support/GrokProxy/config.json`
- Linux：`$XDG_CONFIG_HOME/GrokProxy/config.json` 或 `~/.config/GrokProxy/config.json`

默认回环监听可不设置本地代理密钥；监听 `0.0.0.0`、局域网 IP 或其他非回环地址时，应用会强制要求本地代理密钥。
