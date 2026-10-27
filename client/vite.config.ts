/// <reference types="vitest/config" />
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'

const defaultProxyTarget = 'http://localhost:8080'

// The client finds the server by URL, never by path. In development, /v1
// (including WebSocket upgrades) is proxied to API_PROXY_TARGET, so the
// browser sees one origin. The Host header is passed through unchanged
// (changeOrigin: false) so the server can resolve the tenant from it.
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, import.meta.dirname, '')
  const proxy = {
    '/v1': {
      target: env.API_PROXY_TARGET || defaultProxyTarget,
      changeOrigin: false,
      ws: true,
    },
  }
  // Tenant subdomains such as acme.localhost resolve to this machine.
  const allowedHosts = ['.localhost']

  return {
    plugins: [react(), tailwindcss()],
    server: { port: 5173, strictPort: true, allowedHosts, proxy },
    preview: { allowedHosts, proxy },
    test: { environment: 'happy-dom' },
  }
})
