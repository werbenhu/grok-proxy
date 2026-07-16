# GrokBridge Wails Simplification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the multi-account web gateway with a compact Wails desktop proxy that authenticates to xAI by API key or device OAuth and exposes OpenAI Chat Completions plus Anthropic Messages.

**Architecture:** A Wails `App` delegates to a lifecycle `service`; the service owns a JSON config store, an OAuth token source, an xAI Responses client, and a local `net/http` proxy. Both downstream protocols use the isolated `internal/protocol/conversation` package, while all secrets remain below the service boundary and only masked state reaches the UI.

**Tech Stack:** Go 1.24+, Wails v2.12, standard-library HTTP/JSON, TypeScript 6, Vite 8, pnpm 10+.

## Global Constraints

- Default listen address is exactly `127.0.0.1:8181`.
- Non-loopback listening requires a non-empty local proxy key.
- Maximum downstream request body is exactly 32 MiB.
- Supported downstream inference routes are exactly `POST /v1/chat/completions` and `POST /v1/messages`; model discovery is `GET /v1/models`.
- Upstream API-key base URL is `https://api.x.ai/v1`; upstream OAuth base URL is `https://cli-chat-proxy.grok.com/v1`.
- Configuration is a single atomic JSON file in the OS user config directory and UI-facing values are always masked.
- No database, Redis, account pool, admin login, audit persistence, media generation, Swagger, or multi-instance coordination.

---

### Task 1: Wails skeleton and configuration store

**Files:**
- Create: `go.mod`, `main.go`, `app.go`, `wails.json`
- Create: `internal/config/config.go`, `internal/config/store.go`
- Test: `internal/config/config_test.go`, `internal/config/store_test.go`
- Replace: `.gitignore`

**Interfaces:**
- Produces: `config.Config`, `config.Public`, `config.Default()`, `config.Validate(Config) error`, `config.NewStore(path string)`, `(*Store).Load()`, `(*Store).Save(Config)`, and `(*Store).Current()`.
- `Config` contains `ListenHost string`, `ListenPort int`, `LocalKey string`, `AuthMode string`, `APIKey string`, and `OAuth config.OAuth`.

- [ ] **Step 1: Write failing default, safety, masking, and atomic persistence tests**

```go
func TestValidateRequiresKeyOutsideLoopback(t *testing.T) {
    cfg := Default()
    cfg.ListenHost = "0.0.0.0"
    if err := Validate(cfg); err == nil { t.Fatal("expected validation error") }
}

func TestPublicNeverExposesSecrets(t *testing.T) {
    cfg := Default(); cfg.APIKey = "xai-secret"; cfg.LocalKey = "local-secret"
    encoded, _ := json.Marshal(cfg.Public())
    if bytes.Contains(encoded, []byte("secret")) { t.Fatalf("secret leaked: %s", encoded) }
}
```

- [ ] **Step 2: Run `go test ./internal/config` and verify it fails because the package API is absent**
- [ ] **Step 3: Implement validated defaults, secret masking, locked store access, temp-file sync, rename, and `0600` permissions**
- [ ] **Step 4: Run `go test ./internal/config` and verify all config tests pass**
- [ ] **Step 5: Add the minimal Wails bootstrap using embedded `frontend/dist` assets and run `go test ./...`**

### Task 2: Device OAuth and refreshable token source

**Files:**
- Create: `internal/auth/oauth.go`, `internal/auth/source.go`, `internal/auth/errors.go`
- Test: `internal/auth/oauth_test.go`, `internal/auth/source_test.go`

**Interfaces:**
- Produces: `auth.DeviceAuthorization`, `auth.Token`, `auth.NewOAuthClient(httpClient)`, `(*OAuthClient).Start(ctx)`, `Poll(ctx, deviceCode)`, `Refresh(ctx, refreshToken)`.
- Produces: `auth.NewSource(client, store)`, `(*Source).AccessToken(ctx)`; `TokenStore` exposes `OAuth() config.OAuth` and `SaveOAuth(config.OAuth) error`.

- [ ] **Step 1: Write table-driven failing tests for device start, pending, slow-down, denial, refresh-token fallback, and malformed responses**

```go
func TestPollReportsPending(t *testing.T) {
    server := oauthServer(t, http.StatusBadRequest, `{"error":"authorization_pending"}`)
    client := newTestOAuthClient(server.Client(), server.URL)
    _, err := client.Poll(context.Background(), "device")
    if !errors.Is(err, ErrAuthorizationPending) { t.Fatalf("err=%v", err) }
}
```

- [ ] **Step 2: Run `go test ./internal/auth -run 'Test(Start|Poll|Refresh)'` and confirm the expected missing-symbol failure**
- [ ] **Step 3: Implement OAuth form exchange with 1 MiB response limits and typed pending/slow-down/denied errors**
- [ ] **Step 4: Write a failing concurrency test proving five simultaneous expired-token reads cause one refresh request**
- [ ] **Step 5: Implement double-checked refresh locking, five-minute expiry skew, and atomic token persistence**
- [ ] **Step 6: Run `go test -race ./internal/auth` and verify all auth tests pass**

### Task 3: Extract the pure protocol compatibility core

**Files:**
- Move: `backend/internal/infra/provider/conversation/*.go` to `internal/protocol/conversation/*.go`
- Test: `internal/protocol/conversation/conversation_test.go`

**Interfaces:**
- Produces: `ConvertRequestWithOptions(body []byte, model, operation string) ([]byte, ResponseOptions, error)`.
- Produces: `ConvertResponseJSONWithOptions(body []byte, operation string, options ResponseOptions) ([]byte, error)`.
- Produces: `ConvertResponseStreamWithOptions(source io.ReadCloser, operation string, options ResponseOptions) io.ReadCloser`.

- [ ] **Step 1: Move only `conversation_test.go` and update its package path**
- [ ] **Step 2: Run `go test ./internal/protocol/conversation` and verify it fails because converters are absent**
- [ ] **Step 3: Move the focused converter implementation files without importing any legacy application package**
- [ ] **Step 4: Add failing coverage for an OpenAI streamed function call and Anthropic `message_start → content blocks → message_delta → message_stop` ordering**
- [ ] **Step 5: Make the smallest converter corrections required by those tests**
- [ ] **Step 6: Run `go test -race ./internal/protocol/conversation` and verify all protocol cases pass**

### Task 4: xAI upstream client and local proxy

**Files:**
- Create: `internal/upstream/client.go`, `internal/upstream/credential.go`
- Test: `internal/upstream/client_test.go`
- Create: `internal/proxy/server.go`, `internal/proxy/handlers.go`, `internal/proxy/stats.go`
- Test: `internal/proxy/server_test.go`, `internal/proxy/handlers_test.go`

**Interfaces:**
- `upstream.CredentialSource.Authorization(ctx) (upstream.Authorization, error)` returns mode and token only at request time.
- `(*upstream.Client).Models(ctx)` returns a bounded raw response; `Responses(ctx, body, stream)` returns the live upstream response.
- `proxy.Upstream` mirrors those two operations; `proxy.New(cfg, upstream)` returns an `http.Handler` plus thread-safe stats.

- [ ] **Step 1: Write failing upstream tests for API-key headers, OAuth Grok CLI headers, `/models`, `/responses`, gzip, cancellation, and 2 MiB diagnostic limits**
- [ ] **Step 2: Run `go test ./internal/upstream` and confirm missing implementation failures**
- [ ] **Step 3: Implement the standard-library xAI client with per-request credential resolution and no secret logging**
- [ ] **Step 4: Write failing proxy tests for health, model forwarding, both local-key headers, 32 MiB limit, OpenAI/Anthropic validation shapes, JSON conversion, SSE flush, and client cancellation**

```go
func TestMessagesRequiresAnthropicVersion(t *testing.T) {
    req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"grok-4","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`))
    rec := httptest.NewRecorder(); handler.ServeHTTP(rec, req)
    if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"type":"error"`) { t.Fatalf("%d %s", rec.Code, rec.Body.String()) }
}
```

- [ ] **Step 5: Implement routing, constant-time local-key comparison, bounded bodies, protocol conversion, SSE headers/flushing, and memory-only stats**
- [ ] **Step 6: Run `go test -race ./internal/upstream ./internal/proxy` and verify all tests pass**

### Task 5: Lifecycle service and Wails API

**Files:**
- Create: `internal/service/service.go`, `internal/service/state.go`
- Test: `internal/service/service_test.go`
- Replace: `app.go`
- Test: `app_test.go`

**Interfaces:**
- Produces: `service.New(store, oauthClient, httpClient)`, `Start`, `Stop`, `Restart`, `State`, `Save`, `BeginOAuth`, `CompleteOAuth`, `ClearCredential`, and `TestConnection`.
- Wails exposes `GetState`, `SaveSettings`, `StartProxy`, `StopProxy`, `BeginOAuth`, `CompleteOAuth`, `ClearCredential`, `TestConnection`, and `OpenURL` using public DTOs only.

- [ ] **Step 1: Write failing service tests for auto-start, no-credential waiting, idempotent stop, port-conflict rollback, and OAuth completion**
- [ ] **Step 2: Run `go test ./internal/service` and confirm missing implementation failures**
- [ ] **Step 3: Implement mutex-protected listener lifecycle, state snapshots, credential selection, and restart rollback**
- [ ] **Step 4: Write a failing reflection/JSON test asserting App DTOs never expose `APIKey`, access token, or refresh token fields**
- [ ] **Step 5: Implement the thin Wails facade and URL allowlist for `https://auth.x.ai` and local endpoint documentation**
- [ ] **Step 6: Run `go test -race ./internal/service ./...` and verify the lifecycle and facade tests pass**

### Task 6: Minimal TypeScript desktop UI

**Files:**
- Replace: `frontend/package.json`, `frontend/pnpm-lock.yaml`, `frontend/vite.config.ts`, `frontend/tsconfig.json`, `frontend/index.html`
- Create: `frontend/src/main.ts`, `frontend/src/style.css`, `frontend/src/types.ts`, `frontend/src/api.ts`
- Generate: `frontend/wailsjs/go/main/App.*`, `frontend/wailsjs/runtime/*`

**Interfaces:**
- Consumes only the Wails `main.App` methods from Task 5.
- Renders service status, credential mode, OAuth code flow, proxy settings, endpoint examples, copy actions, and inline errors.

- [ ] **Step 1: Replace the dependency-heavy SPA manifest with Vite and TypeScript only; add `typecheck`, `lint`, and `build` scripts**
- [ ] **Step 2: Implement typed API wrappers and a single state renderer with escaped text insertion and no secret echo**
- [ ] **Step 3: Implement responsive dark desktop styling, disabled/loading states, OAuth user-code panel, and endpoint copy buttons**
- [ ] **Step 4: Run `pnpm install --frozen-lockfile=false`, `pnpm typecheck`, `pnpm lint`, and `pnpm build`; verify all exit zero**
- [ ] **Step 5: Run `wails generate module` and repeat the frontend checks against generated bindings**

### Task 7: Delete legacy systems, document, and release-verify

**Files:**
- Delete: remaining tracked `backend/**` and obsolete tracked `frontend/**`
- Delete: `config.yaml`, `config.example.yaml`, `start.bat`, `_init_secrets.ps1`
- Replace: `README.md`, `Makefile`, `make.bat`, `VERSION`, `.github/workflows/*`
- Create: `docs/api.md`, `build/windows/info.json`, `build/windows/wails.exe.manifest`
- Reuse: existing GrokBridge image as `build/appicon.png`

**Interfaces:**
- Produces documented client examples for OpenAI SDK, Anthropic SDK, curl, and environment variables.

- [ ] **Step 1: Delete all legacy application files only after the new test suite is green; retain LICENSE, new design/plan docs, and ignored local build/data files**
- [ ] **Step 2: Write README and API docs with exact routes, authentication modes, config location, build commands, migration break, and troubleshooting**
- [ ] **Step 3: Add cross-platform Make targets and a Windows build script for `test`, `dev`, and `build`**
- [ ] **Step 4: Run `gofmt -w` on all Go files and verify `git diff --check`**
- [ ] **Step 5: Run `go test -race ./...`, `go vet ./...`, `pnpm --dir frontend typecheck`, `pnpm --dir frontend lint`, and `pnpm --dir frontend build`**
- [ ] **Step 6: Run `wails build -clean` and verify the desktop binary is produced**
- [ ] **Step 7: Start the proxy against a local fake xAI server and verify health, models, OpenAI JSON/SSE, Anthropic JSON/SSE, bad key, and cancellation**
- [ ] **Step 8: Scan tracked source to prove legacy account/database/Redis/admin/media packages are gone, review `git diff --stat` and `git status`, then commit the implementation**

## Plan self-review

- Every design requirement maps to Tasks 1–7; no independent subsystem is deferred.
- All cross-task types are named at their producer and consumer boundaries.
- The protocol extraction preserves the existing high-value compatibility tests while new networking and lifecycle behavior is test-first.
- Deletion is intentionally last so the new implementation remains verifiable throughout the rewrite.
