export type Theme = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'grok-proxy.theme'

export function detectTheme(): Theme {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved === 'light' || saved === 'dark' || saved === 'system') return saved
  return 'system'
}

let current: Theme = detectTheme()

export function getTheme(): Theme {
  return current
}

export function setTheme(theme: Theme) {
  current = theme
  localStorage.setItem(STORAGE_KEY, theme)
  applyTheme()
}

function resolveTheme(): 'light' | 'dark' {
  if (current === 'system') {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }
  return current
}

export function applyTheme() {
  document.documentElement.dataset.theme = resolveTheme()
}

export function initTheme() {
  applyTheme()
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (current === 'system') applyTheme()
  })
}
