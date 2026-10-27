/// <reference types="vitest/config" />
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'

const defaultProxyTarget = 'http://localhost:8080'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, import.meta.dirname, '')
  const proxy = {
    '/v1': {
      target: env.API_PROXY_TARGET || defaultProxyTarget,
      ws: true,
    },
  }

  return {
    plugins: [react(), tailwindcss()],
    server: { port: 5173, strictPort: true, proxy },
    preview: { proxy },
    test: { environment: 'happy-dom' },
  }
})
