export interface PublicConfig {
  listenHost: string
  listenPort: number
  authMode: '' | 'api_key' | 'oauth'
  hasCredential: boolean
  hasApiKey: boolean
  hasOAuth: boolean
  hasLocalKey: boolean
  apiKeyHint?: string
  localKeyHint?: string
  oauthExpires?: string
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

export interface Settings {
  listenHost: string
  listenPort: number
  authMode: string
  apiKey?: string
  localKey?: string
  clearLocalKey?: boolean
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
