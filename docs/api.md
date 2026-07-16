# GrokProxy API

默认服务根地址为 `http://127.0.0.1:8181`，请求体上限为 32 MiB。

## 客户端鉴权

本地代理密钥未设置时，仅回环监听可启动，客户端鉴权关闭。设置本地代理密钥后，以下任一请求头有效：

```http
Authorization: Bearer <local-key>
x-api-key: <local-key>
```

这枚密钥只保护本地代理，不是 xAI API Key。xAI API Key 或 OAuth token 只保存在桌面配置中，不应由客户端传入。

## 健康检查

```http
GET /healthz
```

返回 `{"status":"ok"}`。健康检查不要求本地代理密钥。

## 模型

```http
GET /v1/models
```

响应直接使用 xAI 的 OpenAI 风格模型列表。上游未授权时返回 OpenAI 风格错误。

## OpenAI Chat Completions

```http
POST /v1/chat/completions
Content-Type: application/json
```

最低请求：

```json
{
  "model": "grok-4",
  "messages": [{"role": "user", "content": "Hello"}]
}
```

支持 `stream`、`temperature`、`top_p`、`max_tokens`/`max_completion_tokens`、`stop`、`tools`、`tool_choice`、`parallel_tool_calls`、`response_format`、图片内容块和常用推理参数。流式响应使用 OpenAI `chat.completion.chunk`，最后发送 `data: [DONE]`。

## Anthropic Messages

```http
POST /v1/messages
Content-Type: application/json
anthropic-version: 2023-06-01
```

最低请求：

```json
{
  "model": "grok-4",
  "max_tokens": 512,
  "messages": [{"role": "user", "content": "Hello"}]
}
```

`anthropic-version`、`model`、`max_tokens` 和 `messages` 必填。支持字符串或 text block `system`、图片、`tool_use`/`tool_result`、`tools`、`tool_choice`、`stop_sequences`、`thinking` 与流式响应。

流式事件顺序与 Anthropic Messages 一致：

```text
message_start
content_block_start
content_block_delta ...
content_block_stop
message_delta
message_stop
```

单个上游 SSE 事件上限为 8 MiB。畸形事件、超限事件或缺少完成事件的中断流会直接终止，不会伪造成功结束帧。

## 错误

OpenAI 路由使用：

```json
{"error":{"message":"...","type":"invalid_request_error","code":"..."}}
```

Anthropic 路由使用：

```json
{"type":"error","error":{"type":"invalid_request_error","message":"..."}}
```

上游 401、403、429、超时与服务不可用状态会映射到相应协议错误类型。响应不会包含上游 Authorization、API Key、access token 或 refresh token。
