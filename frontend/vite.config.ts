import { defineConfig } from 'vite'

export default defineConfig({
  clearScreen: false,
  server: { strictPort: true },
  envPrefix: ['VITE_', 'WAILS_'],
  build: { target: 'es2022', outDir: 'dist', emptyOutDir: true },
})
