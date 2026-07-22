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
    <div class="brand">
      <img class="brand-mark" src="/grok-proxy.png" alt="" />
      <div>
        <h1>GrokProxy</h1>
        <p id="endpoint-caption"></p>
      </div>
    </div>
    <div class="service-actions">
      <button id="service-toggle" class="button primary" disabled></button>
      <div class="segmented-switch theme-switch" role="group" aria-label="Theme">
        <button type="button" data-theme="light" title="Light" aria-label="Light">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="5"/><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/></svg>
        </button>
        <button type="button" data-theme="dark" title="Dark" aria-label="Dark">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
        </button>
        <button type="button" data-theme="system" title="System" aria-label="System">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>
        </button>
      </div>
      <div class="segmented-switch lang-switch" role="group" aria-label="Language">
        <button type="button" data-lang="zh">中</button>
        <button type="button" data-lang="en">EN</button>
      </div>
    </div>
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

  <div class="content">
    <article class="panel auth-panel">
      <div class="section-head">
        <h3 data-i18n="connectGrok"></h3>
        <span id="status-pill" class="pill"><i></i><span></span></span>
      </div>

      <div class="auth-tabs" role="tablist" aria-label="Auth method">
        <button type="button" class="auth-tab active" data-auth-tab="device" role="tab" aria-selected="true" data-i18n="deviceAuthTitle"></button>
        <button type="button" class="auth-tab" data-auth-tab="api_key" role="tab" aria-selected="false" data-i18n="apiKeyTitle"></button>
      </div>

      <div class="panel-scroll">
        <div id="auth-pane-api_key" class="auth-pane hidden" role="tabpanel">
          <div class="option-copy">
            <span data-i18n="apiKeyDesc"></span>
          </div>
          <div class="input-row">
            <input id="api-key" type="password" autocomplete="off" placeholder="xai-..." />
            <button id="save-api-key" class="button primary" data-i18n="save"></button>
          </div>
          <small id="api-key-hint"></small>
        </div>

        <div id="auth-pane-device" class="auth-pane" role="tabpanel">
          <div class="option-copy">
            <span data-i18n="deviceAuthDesc"></span>
          </div>
          <button id="oauth-start" class="button secondary wide" data-i18n="startOauth"></button>
        </div>
      </div>

      <div class="inline-actions">
        <button id="test-connection" class="text-button" data-i18n="testConnection"></button>
        <button id="clear-credential" class="text-button danger" data-i18n="clearCreds"></button>
      </div>
    </article>

    <article class="panel settings-panel">
      <div class="section-head">
        <h3 data-i18n="proxySettings"></h3>
      </div>

      <div class="panel-scroll">
        <div class="field-row">
          <label><span data-i18n="listenHost"></span><input id="listen-host" value="127.0.0.1" spellcheck="false" /></label>
          <label class="port"><span data-i18n="port"></span><input id="listen-port" type="number" min="1" max="65535" value="8181" /></label>
        </div>
        <label>
          <span data-i18n="localKey"></span>
          <input id="local-key" type="text" autocomplete="off" spellcheck="false" data-i18n-placeholder="localKeyPlaceholder" />
        </label>
      </div>

      <button id="save-settings" class="button primary wide" data-i18n="saveSettings"></button>
    </article>

    <section class="panel client-panel">
      <div class="section-head">
        <h3>OpenAI SDK</h3>
        <button class="text-button" data-copy="openai" data-i18n="copy"></button>
      </div>
      <pre id="openai-snippet"></pre>
    </section>
    <section class="panel client-panel">
      <div class="section-head">
        <h3>Anthropic SDK</h3>
        <button class="text-button" data-copy="anthropic" data-i18n="copy"></button>
      </div>
      <pre id="anthropic-snippet"></pre>
    </section>
  </div>

  <footer>
    <span><span data-i18n="requests"></span> <b id="total-requests">0</b></span>
    <span><span data-i18n="active"></span> <b id="active-requests">0</b></span>
    <span id="last-request"></span>
    <span class="version">v0.0.10</span>
  </footer>
</main>`

const element = <T extends HTMLElement>(id: string): T => {
  const value = document.getElementById(id)
  if (!value) throw new Error(t('missingElement', { id }))
  return value as T
}

type AuthTab = 'api_key' | 'device'

let state: AppState | undefined
let authorization: DeviceAuthorization | undefined
let busy = false
let authTab: AuthTab = 'device'
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
  element('auth-pane-api_key').classList.toggle('hidden', tab !== 'api_key')
  element('auth-pane-device').classList.toggle('hidden', tab !== 'device')
}

const setBusy = (value: boolean) => {
  busy = value
  document.querySelectorAll<HTMLButtonElement>('button').forEach((button) => {
    if (button.dataset.lang || button.dataset.theme || button.dataset.authTab) return
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

function render(next: AppState) {
  state = next
  applyStaticI18n()

  const pill = element<HTMLSpanElement>('status-pill')
  pill.className = `pill ${next.status === 'reauthorization_required' ? 'failed' : next.config.hasCredential ? 'online' : ''}`
  pill.querySelector('span')!.textContent = statusLabel(next)

  element('endpoint-caption').textContent = next.running ? t('appSubtitleRunning') : t('appSubtitle')

  const toggle = element<HTMLButtonElement>('service-toggle')
  toggle.textContent = next.running ? t('stopProxy') : t('startProxy')
  toggle.className = `button ${next.running ? 'danger' : 'primary'}`
  // Stay clickable without a credential: the click handler shows a toast
  // explaining that Grok must be connected first.
  toggle.disabled = busy

  if (!settingsDirty) {
    element<HTMLInputElement>('listen-host').value = next.config.listenHost
    element<HTMLInputElement>('listen-port').value = String(next.config.listenPort)
    element<HTMLInputElement>('local-key').value = next.config.localKey ?? ''
  }

  element('api-key-hint').textContent = next.config.hasApiKey
    ? t('apiKeySaved', { hint: next.config.apiKeyHint ?? '' })
    : t('apiKeyUnset')

  element('total-requests').textContent = String(next.stats.totalRequests ?? 0)
  element('active-requests').textContent = String(next.stats.activeRequests ?? 0)
  element('last-request').textContent = next.stats.lastRequestAt
    ? t('lastRequest', { time: new Date(next.stats.lastRequestAt).toLocaleTimeString() })
    : t('noRequests')

  const key = next.config.localKey || '<LOCAL_PROXY_KEY>'
  element('openai-snippet').textContent = `OPENAI_BASE_URL=${next.openaiBaseUrl}\nOPENAI_API_KEY=${key}\nOPENAI_MODEL=grok-4.5`
  element('anthropic-snippet').textContent = `ANTHROPIC_BASE_URL=${next.anthropicBaseUrl}\nANTHROPIC_API_KEY=${key}\nANTHROPIC_MODEL=grok-4.5`

  if (next.lastError) showNotice(next.lastError, 'error')
}

function switchLocale(locale: Locale) {
  setLocale(locale)
  if (state) render(state)
  else applyStaticI18n()
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

element('service-toggle').addEventListener('click', () => {
  const wasRunning = !!state?.running
  if (!wasRunning && !state?.config.hasCredential) {
    showNotice(t('connectGrokFirst'), 'error')
    return
  }
  void run(() => (wasRunning ? api.stop() : api.start()), wasRunning ? 'proxyStopped' : 'proxyStarted')
})

element('save-api-key').addEventListener('click', () => {
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
    if (saved) settingsDirty = false
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

element('oauth-start').addEventListener('click', async () => {
  if (busy) return
  switchAuthTab('device')
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
})

element('oauth-cancel').addEventListener('click', () => {
  oauthCancelled = true
  closeOAuthDialog()
})

async function pollOAuth(flow: DeviceAuthorization) {
  const deadline = Date.now() + flow.expiresInSeconds * 1000
  let delay = Math.max(flow.intervalSeconds, 2) * 1000
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
      if (message.includes('轮询过快') || message.toLowerCase().includes('slow down')) {
        delay += 5000
        continue
      }
      if (
        message.includes('等待用户')
        || message.includes('authorization_pending')
        || message.toLowerCase().includes('pending')
      ) {
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

document.querySelectorAll<HTMLButtonElement>('[data-copy]').forEach((button) => {
  button.addEventListener('click', () => {
    const kind = button.dataset.copy
    const value = kind === 'openai'
      ? element('openai-snippet').textContent
      : kind === 'anthropic'
        ? element('anthropic-snippet').textContent
        : state?.address
    if (value) void navigator.clipboard.writeText(value).then(() => showNotice(t('copied')))
  })
})

async function bootstrap() {
  setLocale(getLocale())
  initTheme()
  applyStaticI18n()
  switchAuthTab(authTab)
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
