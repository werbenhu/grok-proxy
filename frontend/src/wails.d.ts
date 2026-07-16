import type { AppState, ConnectionTest, DeviceAuthorization, Settings } from './types'

declare global {
  interface Window {
    go?: {
      main: {
        App: {
          GetState(): Promise<AppState>
          SaveSettings(input: Settings): Promise<AppState>
          StartProxy(): Promise<AppState>
          StopProxy(): Promise<AppState>
          BeginOAuth(): Promise<DeviceAuthorization>
          CompleteOAuth(deviceCode: string): Promise<AppState>
          ClearCredential(): Promise<AppState>
          TestConnection(): Promise<ConnectionTest>
          OpenURL(url: string): Promise<void>
        }
      }
    }
  }
}

export {}
