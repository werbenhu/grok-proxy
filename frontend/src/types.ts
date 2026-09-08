export interface PublicOutboundProxy {
  mode: 'system' | 'direct' | 'http' | 'https' | 'socks5'
  host?: string
  port?: number
  username?: string
  hasPassword: boolean
}

export interface PublicConfig {
  listenHost: string
  listenPort: number
  authMode: '' | 'api_key' | 'oauth'
  hasCredential: boolean
  hasApiKey: boolean
  hasOAuth: boolean
  hasLocalKey: boolean
  apiKeyHint?: string
  localKey?: string
  localKeyHint?: string
  oauthExpires?: string
  outboundProxy: PublicOutboundProxy
}

export interface Statistics {
  totalRequests: number
  activeRequests: number
  lastError?: string
  lastRequestAt?: string
}

export interface AppState {
  config: PublicConfig
  running: boolean
  status: 'waiting' | 'running' | 'stopped' | 'error' | 'reauthorization_required'
  address: string
  openaiBaseUrl: string
  anthropicBaseUrl: string
  lastError?: string
  stats: Statistics
}

export interface OutboundProxySettings {
  mode: PublicOutboundProxy['mode']
  host?: string
  port?: number
  username?: string
  password?: string
}

export interface Settings {
  listenHost: string
  listenPort: number
  authMode: string
  apiKey?: string
  localKey?: string
  outboundProxy?: OutboundProxySettings
}

export interface DeviceAuthorization {
  deviceCode: string
  userCode: string
  verificationUri: string
  verificationUriComplete?: string
  intervalSeconds: number
  expiresInSeconds: number
}

export interface ConnectionTest { ok: boolean; latencyMs: number; message: string }
