import type { AppState, ConnectionTest, DeviceAuthorization, Settings } from './types'

function app() {
  const binding = window.go?.main.App
  if (!binding) throw new Error('Wails 运行时尚未就绪')
  return binding
}

export const api = {
  state: (): Promise<AppState> => app().GetState(),
  save: (settings: Settings): Promise<AppState> => app().SaveSettings(settings),
  start: (): Promise<AppState> => app().StartProxy(),
  stop: (): Promise<AppState> => app().StopProxy(),
  beginOAuth: (): Promise<DeviceAuthorization> => app().BeginOAuth(),
  completeOAuth: (deviceCode: string): Promise<AppState> => app().CompleteOAuth(deviceCode),
  clearCredential: (): Promise<AppState> => app().ClearCredential(),
  test: (): Promise<ConnectionTest> => app().TestConnection(),
  openURL: (url: string): Promise<void> => app().OpenURL(url),
}
