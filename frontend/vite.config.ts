import { defineConfig } from 'vite'

export default defineConfig({
  clearScreen: false,
  server: { strictPort: true, port: 5273 },
  envPrefix: ['VITE_', 'WAILS_'],
  build: { target: 'es2022', outDir: 'dist', emptyOutDir: true },
})
