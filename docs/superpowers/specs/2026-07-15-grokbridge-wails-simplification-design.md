# GrokBridge Wails 精简重构设计

## 目标

把现有多账号 Web 管理网关重写为一个本地优先的 Wails 桌面代理。产品只解决一件事：使用 xAI API Key 或 Grok/xAI 设备授权连接 Grok，并在本机提供 OpenAI Chat Completions 与 Anthropic Messages 兼容接口。

## 方案选择

评估过三种方案：

1. 在旧 Go 服务外增加 Wails 壳。迁移风险较低，但账号、数据库、Redis、管理鉴权、审计和媒体等复杂度全部保留，不符合精简目标。
2. 只代理官方 xAI API Key。代码最少，但无法满足“Grok 授权或 API Key”两种连接方式。
3. 收缩式重写为 Wails 桌面壳、单一代理服务和双凭据上游。协议转换复用现有已经验证的纯 Go 对话转换逻辑，其余模块重写。

采用方案 3。它保留必要的协议兼容深度，同时移除与单用户桌面代理无关的运行时设施。

## 产品范围

桌面界面提供：

- 服务启停、运行状态和本地端点展示。
- 监听地址、端口、可选本地代理密钥设置。
- xAI API Key 保存、遮罩显示、连通性测试和清除。
- xAI/Grok OAuth Device Flow：发起授权、打开验证页、显示用户码、等待完成、自动刷新和退出。
- OpenAI 与 Anthropic 客户端配置示例和一键复制。
- 最小请求统计：仅保存在内存中的总请求数、活动请求数和最近错误，不保留提示词或响应正文。

明确不再提供：管理员账号、用户账号、多上游账号池、客户端密钥管理、数据库、Redis、配额调度、审计日志、媒体生成/图库、Swagger、Dashboard、代理节点池、批量导入和多实例部署。

## 外部接口

HTTP 服务默认监听 `127.0.0.1:8181`，提供：

- `GET /healthz`：进程与凭据状态。
- `GET /v1/models`：转发上游模型列表。
- `POST /v1/chat/completions`：OpenAI Chat Completions 兼容，支持非流式、SSE、工具调用、图像输入和推理字段。
- `POST /v1/messages`：Anthropic Messages 兼容，支持非流式、SSE、system、工具调用、图片和 thinking。

本地代理密钥为空时不校验客户端凭据，但只允许监听回环地址。非回环监听必须设置本地代理密钥。设置后同时接受 `Authorization: Bearer <key>` 和 Anthropic 的 `x-api-key: <key>`。这是一枚可选的共享密钥，不引入账号、会话或权限模型。

请求体上限固定为 32 MiB。服务设置读取头超时、空闲超时和上游握手超时，但不对生成流设置短的总超时。错误分别编码为 OpenAI 或 Anthropic 标准错误结构，且不会把上游凭据写入响应或日志。

## 上游与凭据

两种凭据实现同一个 `CredentialSource` 接口：

- API Key 模式：使用 `https://api.x.ai/v1`，发送标准 Bearer API Key。
- OAuth 模式：使用 xAI Device Authorization，保存 access token、refresh token 和过期时间；请求发送到 `https://cli-chat-proxy.grok.com/v1`，附带 Grok CLI 所需的稳定客户端头；令牌在过期前自动刷新。

两种模式都以 xAI Responses API 作为内部上游协议。API Key 或 OAuth 只能有一个当前生效，保存某一模式会切换当前模式。OAuth 配置损坏、授权被撤销或刷新永久失败时，服务停止向上游发送请求并在 UI 中显示需要重新授权。

配置保存到操作系统用户配置目录下的 `GrokBridge/config.json`，使用临时文件加原子替换并设置仅当前用户可读写的文件权限。配置文件不包含管理员密码或加密主密钥。桌面应用和代理响应只返回遮罩后的凭据摘要。

## 模块边界

- `internal/config`：配置默认值、校验、原子持久化和对外脱敏。
- `internal/auth`：Device OAuth、刷新和并发安全的令牌读取。
- `internal/upstream`：构造 xAI 请求头、调用 Models/Responses、限制响应诊断体。
- `internal/protocol`：OpenAI/Anthropic 与 Responses 之间的纯数据转换，包括 SSE 转换；不读取配置，不执行网络请求。
- `internal/proxy`：路由、可选本地密钥、错误映射、流式拷贝和请求统计。
- `internal/service`：配置、凭据、上游和 HTTP 服务的生命周期编排。
- 根目录 `App`：仅暴露 Wails UI 所需的方法，不包含协议或网络实现。
- `frontend`：无框架状态层的 TypeScript 单页界面，只通过 Wails 绑定调用 `App`。

依赖方向固定为：Wails App → service → proxy/upstream/auth/config；proxy → protocol + upstream；protocol 不依赖其他内部模块。接口在消费端定义，以便用内存替身测试。

## 生命周期与数据流

应用启动时加载配置并构造服务。配置有效且存在凭据时自动启动本地 HTTP 监听；没有凭据时 UI 可用但代理保持“等待配置”。保存配置会先校验，再原子落盘，然后以新配置重启 HTTP 服务。端口占用等启动失败不会覆盖上一份有效运行配置。

推理请求依次经过请求体限制、可选共享密钥校验、协议校验与转换、凭据读取/刷新、xAI Responses 请求、协议响应转换。流式响应逐事件转换并及时 flush，不缓冲整个生成结果。客户端取消会通过请求 context 取消上游请求。

## 界面设计

界面采用单窗口控制台布局，不使用路由和管理后台导航。顶部显示服务状态、端点和启停按钮；主体为三个卡片：连接 Grok、代理设置、客户端接入。高级字段只包含监听地址、端口和本地代理密钥。危险凭据默认使用密码输入框，UI 状态和错误文案使用中文。

视觉上使用深色中性背景、青绿色状态强调和紧凑的桌面密度。窗口最小尺寸保证端点与操作按钮不发生遮挡；窄窗口下卡片改为单列。

## 测试与验收

- `config`：默认值、回环/非回环安全校验、原子保存、凭据脱敏。
- `auth`：设备授权、pending/slow_down/denied、刷新令牌回退、并发刷新只发生一次。
- `protocol`：复用并扩展现有 Chat Completions 与 Messages 的请求、响应、SSE 和工具调用测试。
- `upstream`：API Key/OAuth 请求头、Models/Responses 路径、取消传播和错误体上限。
- `proxy`：全部路由、两种本地密钥头、协议错误形状、非流式和流式端到端代理。
- `service/App`：启动、停止、重配失败回滚、OAuth 完成后生效、状态脱敏。
- 前端：TypeScript 编译、生产构建和 ESLint。
- 桌面：`go test ./...`、`go vet ./...`、`pnpm build`、`wails build`，并用本地假上游执行 HTTP 冒烟测试。

验收结果必须是：仓库不再编译或引用旧账号系统；根目录是标准 Wails v2 工程；两套兼容接口在 API Key 与 OAuth 凭据抽象下使用同一上游管线；无凭据、无效凭据、端口冲突和客户端取消都有明确行为。

## 迁移

这是破坏式重构，不迁移旧数据库、账号、审计或运行时配置。旧 `config.yaml` 不再读取；新的用户配置首次启动自动生成。README 明确说明旧版部署配置不可复用。构建目录与本机数据不纳入版本控制，也不会由重构脚本删除。
