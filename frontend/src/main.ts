import './style.css'
import { api } from './api'
import { getLocale, setLocale, t, type Locale } from './i18n'
import { getTheme, setTheme, initTheme, type Theme } from './theme'
import type { AppState, DeviceAuthorization } from './types'

const root = document.querySelector<HTMLDivElement>('#root')
if (!root) throw new Error(t('missingRoot'))

root.innerHTML = `
<main class="shell">
  <header class="topbar">
    <button id="service-toggle" class="button primary compact" disabled></button>
    <span id="status-pill" class="pill"><i></i><span></span></span>
    <div class="spacer"></div>
    <div class="segmented-switch theme-switch" role="group" aria-label="Theme">
      <button type="button" data-theme="light" title="Light" aria-label="Light">
        <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="5"/><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/></svg>
      </button>
      <button type="button" data-theme="dark" title="Dark" aria-label="Dark">
        <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
      </button>
      <button type="button" data-theme="system" title="System" aria-label="System">
        <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>
      </button>
    </div>
    <div class="lang-toggle" role="group" aria-label="Language">
      <button type="button" data-lang="zh">中</button>
      <button type="button" data-lang="en">EN</button>
    </div>
    <button id="open-settings" class="icon-button" title="设置" aria-label="设置">
      <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
    </button>
  </header>

  <div id="notice" class="notice hidden" role="status"></div>

  <div id="oauth-box" class="oauth-overlay hidden" role="dialog" aria-modal="true">
    <div class="oauth-card">
      <span data-i18n="oauthHint"></span>
      <strong id="oauth-code">—</strong>
      <div class="oauth-actions">
        <button id="copy-code" class="text-button" data-i18n="copyCode"></button>
        <button id="open-oauth" class="text-button" data-i18n="openOauth"></button>
      </div>
      <div id="oauth-progress" class="progress"><span></span></div>
      <small id="oauth-status"></small>
      <button id="oauth-cancel" class="text-button" data-i18n="cancel"></button>
    </div>
  </div>

  <div id="settings-box" class="settings-overlay hidden" role="dialog" aria-modal="true" aria-labelledby="settings-title">
    <div class="settings-card">
      <div class="settings-head">
        <h3 id="settings-title" data-i18n="proxySettings"></h3>
        <button id="close-settings" class="icon-button" title="关闭" aria-label="关闭">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
        </button>
      </div>
      <div class="settings-body">
        <label class="field">
          <span data-i18n="listenHost"></span>
          <input id="listen-host" value="127.0.0.1" spellcheck="false" />
        </label>
        <label class="field">
          <span data-i18n="port"></span>
          <input id="listen-port" type="number" min="1" max="65535" value="8181" />
        </label>
        <label class="field">
          <span data-i18n="localKey"></span>
          <input id="local-key" type="text" autocomplete="off" spellcheck="false" data-i18n-placeholder="localKeyPlaceholder" />
        </label>
      </div>
      <button id="save-settings" class="button primary wide" data-i18n="saveSettings"></button>
    </div>
  </div>

  <div class="content">
    <article class="panel auth-panel">
      <div class="auth-tabs" role="tablist" aria-label="Auth method">
        <button type="button" class="auth-tab active" data-auth-tab="device" role="tab" aria-selected="true" data-i18n="deviceAuthTitle"></button>
        <button type="button" class="auth-tab" data-auth-tab="api_key" role="tab" aria-selected="false" data-i18n="apiKeyTitle"></button>
      </div>

      <div class="auth-body">
        <div id="auth-pane-device" class="auth-pane" role="tabpanel">
          <div class="option-copy">
            <span data-i18n="deviceAuthDesc"></span>
          </div>
        </div>
        <div id="auth-pane-api_key" class="auth-pane is-inactive" role="tabpanel" aria-hidden="true">
          <div class="input-row">
            <input id="api-key" type="password" autocomplete="off" placeholder="xai-..." />
          </div>
        </div>
      </div>

      <div class="inline-actions">
        <button id="auth-action" class="button secondary"></button>
        <button id="test-connection" class="text-button" data-i18n="testConnection"></button>
        <button id="clear-credential" class="text-button danger" data-i18n="clearCreds"></button>
      </div>
    </article>

    <section id="sdk-strip" class="sdk-strip">
      <div class="sdk-head">
        <div class="seg" role="group" aria-label="SDK protocol">
          <button type="button" class="seg-btn active" data-sdk="openai">OpenAI</button>
          <button type="button" class="seg-btn" data-sdk="anthropic">Anthropic</button>
        </div>
        <button type="button" id="sdk-copy" class="sdk-copy-hint" data-i18n="copy"></button>
      </div>
      <div id="sdk-body" class="sdk-body">
        <pre id="sdk-snippet" class="sdk-snippet"></pre>
      </div>
    </section>
  </div>
</main>`

const element = <T extends HTMLElement>(id: string): T => {
  const value = document.getElementById(id)
  if (!value) throw new Error(t('missingElement', { id }))
  return value as T
}

type AuthTab = 'api_key' | 'device'
type SdkKind = 'openai' | 'anthropic'

let state: AppState | undefined
let authorization: DeviceAuthorization | undefined
let busy = false
let authTab: AuthTab = 'device'
let sdkKind: SdkKind = 'openai'
let settingsOpen = false
// Tracks real user edits to the settings fields so state polling does not
// clobber an unsaved draft. Value comparison against the DOM cannot be used:
// on first render the inputs still hold their hardcoded HTML defaults, which
// differ from the loaded config and would wrongly count as a draft.
let settingsDirty = false

function switchAuthTab(tab: AuthTab) {
  authTab = tab
  document.querySelectorAll<HTMLButtonElement>('[data-auth-tab]').forEach((button) => {
    const active = button.dataset.authTab === tab
    button.classList.toggle('active', active)
    button.setAttribute('aria-selected', active ? 'true' : 'false')
  })
  // Keep both panes in the layout stack so tab height stays identical.
  const devicePane = element('auth-pane-device')
  const apiKeyPane = element('auth-pane-api_key')
  devicePane.classList.toggle('is-inactive', tab !== 'device')
  apiKeyPane.classList.toggle('is-inactive', tab !== 'api_key')
  devicePane.setAttribute('aria-hidden', tab === 'device' ? 'false' : 'true')
  apiKeyPane.setAttribute('aria-hidden', tab === 'api_key' ? 'false' : 'true')

  const action = element<HTMLButtonElement>('auth-action')
  action.textContent = tab === 'api_key' ? t('save') : t('startOauth')
  action.className = `button ${tab === 'api_key' ? 'primary' : 'secondary'}`
}

function switchSdkKind(kind: SdkKind) {
  sdkKind = kind
  document.querySelectorAll<HTMLButtonElement>('[data-sdk]').forEach((button) => {
    button.classList.toggle('active', button.dataset.sdk === kind)
  })
  if (state) renderSdkSnippet()
}

function copySdkSnippet() {
  const text = element('sdk-snippet').textContent ?? ''
  void navigator.clipboard.writeText(text).then(() => showNotice(t('copied')))
}

function openSettings() {
  if (settingsOpen) return
  settingsOpen = true
  element('settings-box').classList.remove('hidden')
}

function closeSettings() {
  if (!settingsOpen) return
  settingsOpen = false
  element('settings-box').classList.add('hidden')
}

const setBusy = (value: boolean) => {
  busy = value
  document.querySelectorAll<HTMLButtonElement>('button').forEach((button) => {
    if (button.dataset.lang || button.dataset.theme || button.dataset.authTab || button.dataset.sdk) return
    button.disabled = value
  })
}

const showNotice = (message: string, kind: 'ok' | 'error' = 'ok') => {
  const notice = element<HTMLDivElement>('notice')
  notice.textContent = message
  notice.className = `notice ${kind}`
  window.setTimeout(() => {
    if (notice.textContent === message) notice.classList.add('hidden')
  }, 5000)
}

const errorMessage = (error: unknown) => error instanceof Error ? error.message : String(error)

function applyStaticI18n() {
  document.querySelectorAll<HTMLElement>('[data-i18n]').forEach((node) => {
    const key = node.dataset.i18n
    if (!key) return
    node.textContent = t(key as Parameters<typeof t>[0])
  })
  document.querySelectorAll<HTMLInputElement>('[data-i18n-placeholder]').forEach((node) => {
    const key = node.dataset.i18nPlaceholder
    if (!key) return
    node.placeholder = t(key as Parameters<typeof t>[0])
  })
  document.querySelectorAll<HTMLButtonElement>('[data-lang]').forEach((button) => {
    button.classList.toggle('active', button.dataset.lang === getLocale())
  })
  document.querySelectorAll<HTMLButtonElement>('[data-theme]').forEach((button) => {
    button.classList.toggle('active', button.dataset.theme === getTheme())
  })
}

function statusLabel(next: AppState): string {
  if (next.status === 'reauthorization_required') return t('statusReauth')
  // "Authorized" means holding a valid Grok credential; it is independent of
  // whether the local proxy is running.
  if (next.config.hasCredential) return t('statusConnected')
  return t('statusWaiting')
}

function renderSdkSnippet() {
  if (!state) return
  const key = state.config.localKey || '<LOCAL_PROXY_KEY>'
  const snippet = sdkKind === 'openai'
    ? `OPENAI_BASE_URL=${state.openaiBaseUrl}\nOPENAI_API_KEY=${key}\nOPENAI_MODEL=grok-4.5`
    : `ANTHROPIC_BASE_URL=${state.anthropicBaseUrl}\nANTHROPIC_API_KEY=${key}\nANTHROPIC_MODEL=grok-4.5`
  element('sdk-snippet').textContent = snippet
}

function render(next: AppState) {
  state = next
  applyStaticI18n()
  // Re-apply tab-dependent action label after static i18n pass.
  switchAuthTab(authTab)

  const pill = element<HTMLSpanElement>('status-pill')
  pill.className = `pill ${next.status === 'reauthorization_required' ? 'failed' : next.config.hasCredential ? 'online' : ''}`
  pill.querySelector('span')!.textContent = statusLabel(next)

  const toggle = element<HTMLButtonElement>('service-toggle')
  toggle.textContent = next.running ? t('stopProxy') : t('startProxy')
  toggle.className = `button compact ${next.running ? 'danger' : 'primary'}`
  // Stay clickable without a credential: the click handler shows a toast
  // explaining that Grok must be connected first.
  toggle.disabled = busy

  if (!settingsDirty) {
    element<HTMLInputElement>('listen-host').value = next.config.listenHost
    element<HTMLInputElement>('listen-port').value = String(next.config.listenPort)
    element<HTMLInputElement>('local-key').value = next.config.localKey ?? ''
  }

  renderSdkSnippet()

  if (next.lastError) showNotice(next.lastError, 'error')
}

function switchLocale(locale: Locale) {
  setLocale(locale)
  // 同步给 Go 端，更新系统托盘菜单文案。
  void api.setLocale(locale)
  if (state) render(state)
  else {
    applyStaticI18n()
    switchAuthTab(authTab)
  }
}

function switchTheme(theme: Theme) {
  setTheme(theme)
  applyStaticI18n()
}

async function run(action: () => Promise<AppState>, successKey: Parameters<typeof t>[0]): Promise<boolean> {
  if (busy) return false
  setBusy(true)
  try {
    render(await action())
    showNotice(t(successKey))
    return true
  } catch (error) {
    showNotice(errorMessage(error), 'error')
    return false
  } finally {
    setBusy(false)
    if (state) render(state)
  }
}

document.querySelectorAll<HTMLButtonElement>('[data-lang]').forEach((button) => {
  button.addEventListener('click', () => {
    const locale = button.dataset.lang
    if (locale === 'zh' || locale === 'en') switchLocale(locale)
  })
})

document.querySelectorAll<HTMLButtonElement>('[data-theme]').forEach((button) => {
  button.addEventListener('click', () => {
    const theme = button.dataset.theme
    if (theme === 'light' || theme === 'dark' || theme === 'system') switchTheme(theme)
  })
})

document.querySelectorAll<HTMLButtonElement>('[data-auth-tab]').forEach((button) => {
  button.addEventListener('click', () => {
    const tab = button.dataset.authTab
    if (tab === 'api_key' || tab === 'device') switchAuthTab(tab)
  })
})

document.querySelectorAll<HTMLButtonElement>('[data-sdk]').forEach((button) => {
  button.addEventListener('click', (event) => {
    event.stopPropagation()
    const kind = button.dataset.sdk
    if (kind === 'openai' || kind === 'anthropic') switchSdkKind(kind)
  })
})

element('service-toggle').addEventListener('click', () => {
  const wasRunning = !!state?.running
  if (!wasRunning && !state?.config.hasCredential) {
    showNotice(t('connectGrokFirst'), 'error')
    return
  }
  void run(() => (wasRunning ? api.stop() : api.start()), wasRunning ? 'proxyStopped' : 'proxyStarted')
})

function saveApiKey() {
  const input = element<HTMLInputElement>('api-key')
  const key = input.value.trim()
  if (!key) {
    showNotice(t('needApiKey'), 'error')
    return
  }
  void run(
    () => api.save({
      listenHost: state!.config.listenHost,
      listenPort: state!.config.listenPort,
      authMode: 'api_key',
      apiKey: key,
    }),
    'apiKeySavedNotice',
  )
  input.value = ''
}

async function startDeviceAuth() {
  if (busy) return
  setBusy(true)
  try {
    authorization = await api.beginOAuth()
    oauthCancelled = false
    element('oauth-code').textContent = authorization.userCode
    element('oauth-status').textContent = t('oauthWaiting')
    setOAuthProgress(true)
    element('oauth-box').classList.remove('hidden')
    await api.openURL(authorization.verificationUriComplete || authorization.verificationUri)
    void pollOAuth(authorization)
  } catch (error) {
    closeOAuthDialog()
    showNotice(errorMessage(error), 'error')
  }
}

// Shared bottom action slot: label/style follow the active tab, height stays fixed.
element('auth-action').addEventListener('click', () => {
  if (authTab === 'api_key') saveApiKey()
  else void startDeviceAuth()
})

element('api-key').addEventListener('keydown', (event) => {
  if (event.key === 'Enter') {
    event.preventDefault()
    saveApiKey()
  }
})

// Any keystroke in the settings fields marks a draft; polling keeps it
// untouched until the save succeeds (which clears the flag below).
for (const id of ['listen-host', 'listen-port', 'local-key']) {
  element<HTMLInputElement>(id).addEventListener('input', () => {
    settingsDirty = true
  })
}

element('save-settings').addEventListener('click', () => {
  const localKeyValue = element<HTMLInputElement>('local-key').value.trim()
  if (!localKeyValue) {
    showNotice(t('needLocalKey'), 'error')
    return
  }
  void run(
    () => api.save({
      listenHost: element<HTMLInputElement>('listen-host').value.trim(),
      listenPort: Number(element<HTMLInputElement>('listen-port').value),
      authMode: state?.config.authMode ?? '',
      localKey: localKeyValue,
    }),
    'settingsSaved',
  ).then((saved) => {
    if (saved) {
      settingsDirty = false
      closeSettings()
    }
  })
})

function setOAuthProgress(visible: boolean) {
  element('oauth-progress').classList.toggle('hidden', !visible)
}

let oauthCancelled = false

function closeOAuthDialog() {
  element('oauth-box').classList.add('hidden')
  setOAuthProgress(false)
  setBusy(false)
  if (state) render(state)
}

element('oauth-cancel').addEventListener('click', () => {
  oauthCancelled = true
  closeOAuthDialog()
})

function isOAuthSlowDown(message: string): boolean {
  const lower = message.toLowerCase()
  return lower.includes('too fast')
    || lower.includes('slow down')
    || lower.includes('slow_down')
    || message.includes('轮询过快')
}

function isOAuthPending(message: string): boolean {
  const lower = message.toLowerCase()
  return lower.includes('waiting for user')
    || lower.includes('authorization_pending')
    || lower.includes('authorization pending')
    || (lower.includes('pending') && lower.includes('authoriz'))
    || message.includes('等待用户')
}

async function pollOAuth(flow: DeviceAuthorization) {
  // Prefer server-provided lifetime; floor at 30 minutes so a short/missing
  // expires_in does not abort the device flow while the user is still logging in.
  const waitSeconds = Math.max(Number(flow.expiresInSeconds) || 0, 30 * 60)
  const deadline = Date.now() + waitSeconds * 1000
  let delay = Math.max(Number(flow.intervalSeconds) || 0, 2) * 1000
  while (Date.now() < deadline) {
    await new Promise((resolve) => window.setTimeout(resolve, delay))
    if (oauthCancelled) return
    try {
      render(await api.completeOAuth(flow.deviceCode))
      closeOAuthDialog()
      showNotice(t('oauthCompleted'))
      return
    } catch (error) {
      const message = errorMessage(error)
      if (isOAuthSlowDown(message)) {
        delay += 5000
        continue
      }
      if (isOAuthPending(message)) {
        continue
      }
      closeOAuthDialog()
      showNotice(message, 'error')
      return
    }
  }
  closeOAuthDialog()
  showNotice(t('oauthExpired'), 'error')
}

element('open-oauth').addEventListener('click', () => {
  if (authorization) void api.openURL(authorization.verificationUriComplete || authorization.verificationUri)
})

element('copy-code').addEventListener('click', () => {
  if (authorization) {
    void navigator.clipboard.writeText(authorization.userCode).then(() => showNotice(t('codeCopied')))
  }
})

element('clear-credential').addEventListener('click', () => {
  void run(() => api.clearCredential(), 'credsCleared')
})

element('test-connection').addEventListener('click', async () => {
  if (busy) return
  setBusy(true)
  try {
    const result = await api.test()
    showNotice(`${result.message} · ${result.latencyMs} ms`)
  } catch (error) {
    showNotice(errorMessage(error), 'error')
  } finally {
    setBusy(false)
    if (state) render(state)
  }
})

// SDK strip stays expanded; only protocol switch + copy are interactive.
element('sdk-copy').addEventListener('click', () => {
  copySdkSnippet()
})

element('open-settings').addEventListener('click', openSettings)
element('close-settings').addEventListener('click', closeSettings)
element('settings-box').addEventListener('click', (event) => {
  if (event.target === event.currentTarget) closeSettings()
})
window.addEventListener('keydown', (event) => {
  if (event.key === 'Escape' && settingsOpen) closeSettings()
})

async function bootstrap() {
  setLocale(getLocale())
  // 启动时把持久化的语言同步给 Go 端托盘菜单。
  void api.setLocale(getLocale())
  initTheme()
  applyStaticI18n()
  switchAuthTab(authTab)
  switchSdkKind(sdkKind)
  try {
    render(await api.state())
    window.setInterval(async () => {
      if (!busy) {
        try { render(await api.state()) } catch { /* app is closing */ }
      }
    }, 2000)
  } catch (error) {
    showNotice(errorMessage(error), 'error')
  }
}

void bootstrap()
