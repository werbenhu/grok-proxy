export type Locale = 'zh' | 'en'

export type MessageKey =
  | 'appSubtitle'
  | 'appSubtitleRunning'
  | 'statusConnected'
  | 'statusWaiting'
  | 'statusReauth'
  | 'startProxy'
  | 'stopProxy'
  | 'connectGrokFirst'
  | 'connectGrok'
  | 'apiKeyTitle'
  | 'apiKeyDesc'
  | 'save'
  | 'apiKeyUnset'
  | 'apiKeySaved'
  | 'or'
  | 'deviceAuthTitle'
  | 'deviceAuthDesc'
  | 'startOauth'
  | 'oauthHint'
  | 'copyCode'
  | 'openOauth'
  | 'oauthWaiting'
  | 'cancel'
  | 'testConnection'
  | 'clearCreds'
  | 'proxySettings'
  | 'listenHost'
  | 'port'
  | 'localKey'
  | 'localKeyPlaceholder'
  | 'needLocalKey'
  | 'saveSettings'
  | 'copy'
  | 'requests'
  | 'active'
  | 'noRequests'
  | 'lastRequest'
  | 'copied'
  | 'needApiKey'
  | 'apiKeySavedNotice'
  | 'settingsSaved'
  | 'proxyStarted'
  | 'proxyStopped'
  | 'oauthCompleted'
  | 'oauthExpired'
  | 'credsCleared'
  | 'codeCopied'
  | 'missingRoot'
  | 'missingElement'
  | 'themeLight'
  | 'themeDark'
  | 'themeSystem'
  | 'themeLabel'

const zh: Record<MessageKey, string> = {
  appSubtitle: 'Grok 本地兼容代理',
  appSubtitleRunning: '代理已启动',
  statusConnected: '已授权',
  statusWaiting: '未授权',
  statusReauth: '需重新授权',
  startProxy: '启动代理',
  stopProxy: '停止代理',
  connectGrokFirst: '请先完成 Grok 授权，再启动代理',
  connectGrok: 'Grok 授权',
  apiKeyTitle: 'xAI API Key',
  apiKeyDesc: '直接使用 api.x.ai，适合官方开发者密钥',
  save: '保存',
  apiKeyUnset: '尚未保存',
  apiKeySaved: '已保存 {hint}',
  or: '或',
  deviceAuthTitle: '网站授权',
  deviceAuthDesc: '通过 xAI 官方网站登录授权，无需复制 Cookie',
  startOauth: '开始 Grok 授权',
  oauthHint: '在打开的页面输入授权码',
  copyCode: '复制授权码',
  openOauth: '打开授权页',
  oauthWaiting: '等待授权确认…',
  cancel: '取消',
  testConnection: '测试连接',
  clearCreds: '清除凭据',
  proxySettings: '代理设置',
  listenHost: '监听地址',
  port: '端口',
  localKey: '本地代理密钥',
  localKeyPlaceholder: '16 位本地密钥',
  needLocalKey: '本地代理密钥不能为空',
  saveSettings: '保存代理设置',
  copy: '复制',
  requests: '请求',
  active: '活动',
  noRequests: '尚无请求',
  lastRequest: '最近请求 {time}',
  copied: '已复制到剪贴板',
  needApiKey: '请输入 xAI API Key',
  apiKeySavedNotice: 'API Key 已保存',
  settingsSaved: '代理设置已保存',
  proxyStarted: '代理已启动',
  proxyStopped: '代理已停止',
  oauthCompleted: 'Grok 授权已完成',
  oauthExpired: '授权已过期，请重新开始',
  credsCleared: 'Grok 凭据已清除',
  codeCopied: '授权码已复制',
  missingRoot: '缺少应用根节点',
  missingElement: '缺少界面元素 {id}',
  themeLight: '浅色',
  themeDark: '深色',
  themeSystem: '跟随系统',
  themeLabel: '主题',
}

const en: Record<MessageKey, string> = {
  appSubtitle: 'Local Grok-compatible proxy',
  appSubtitleRunning: 'Proxy started',
  statusConnected: 'Authorized',
  statusWaiting: 'Not authorized',
  statusReauth: 'Reauth needed',
  startProxy: 'Start proxy',
  stopProxy: 'Stop proxy',
  connectGrokFirst: 'Authorize Grok before starting the proxy',
  connectGrok: 'Grok Authorization',
  apiKeyTitle: 'xAI API Key',
  apiKeyDesc: 'Use api.x.ai directly with an official developer key',
  save: 'Save',
  apiKeyUnset: 'Not saved yet',
  apiKeySaved: 'Saved {hint}',
  or: 'or',
  deviceAuthTitle: 'Site auth',
  deviceAuthDesc: 'Sign in via the xAI official website — no cookie copy',
  startOauth: 'Start Grok auth',
  oauthHint: 'Enter this code on the opened page',
  copyCode: 'Copy code',
  openOauth: 'Open auth page',
  oauthWaiting: 'Waiting for confirmation…',
  cancel: 'Cancel',
  testConnection: 'Test connection',
  clearCreds: 'Clear credentials',
  proxySettings: 'Proxy settings',
  listenHost: 'Listen host',
  port: 'Port',
  localKey: 'Local proxy key',
  localKeyPlaceholder: '16-char local key',
  needLocalKey: 'Local proxy key cannot be empty',
  saveSettings: 'Save proxy settings',
  copy: 'Copy',
  requests: 'Requests',
  active: 'Active',
  noRequests: 'No requests yet',
  lastRequest: 'Last request {time}',
  copied: 'Copied to clipboard',
  needApiKey: 'Enter an xAI API Key',
  apiKeySavedNotice: 'API Key saved',
  settingsSaved: 'Proxy settings saved',
  proxyStarted: 'Proxy started',
  proxyStopped: 'Proxy stopped',
  oauthCompleted: 'Grok authorization completed',
  oauthExpired: 'Authorization expired — please start again',
  credsCleared: 'Grok credentials cleared',
  codeCopied: 'Auth code copied',
  missingRoot: 'Missing app root',
  missingElement: 'Missing UI element {id}',
  themeLight: 'Light',
  themeDark: 'Dark',
  themeSystem: 'System',
  themeLabel: 'Theme',
}

const catalogs: Record<Locale, Record<MessageKey, string>> = { zh, en }

const STORAGE_KEY = 'grok-proxy.locale'

export function detectLocale(): Locale {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved === 'zh' || saved === 'en') return saved
  return navigator.language.toLowerCase().startsWith('zh') ? 'zh' : 'en'
}

let current: Locale = detectLocale()

export function getLocale(): Locale {
  return current
}

export function setLocale(locale: Locale) {
  current = locale
  localStorage.setItem(STORAGE_KEY, locale)
  document.documentElement.lang = locale === 'zh' ? 'zh-CN' : 'en'
}

export function t(key: MessageKey, vars?: Record<string, string | number>): string {
  let text = catalogs[current][key] ?? catalogs.en[key] ?? key
  if (vars) {
    for (const [name, value] of Object.entries(vars)) {
      text = text.replaceAll(`{${name}}`, String(value))
    }
  }
  return text
}
