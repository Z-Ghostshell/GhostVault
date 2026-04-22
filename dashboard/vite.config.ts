import path from 'node:path'
import { fileURLToPath } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const repoEnvDir = path.resolve(__dirname, '..')

/**
 * Dev proxy for `/v1` → gvsvd. Uses `GHOSTVAULT_BASE_URL` (or `*_PROXY_URL`) from repo `.env`.
 * If the URL has a path (e.g. `http://127.0.0.1:8989/api` from Docker edge), rewrites `/v1/…` → `/api/v1/…`.
 */
function resolveV1Proxy(env: Record<string, string>): { target: string; rewrite: (path: string) => string } {
  const raw =
    process.env.VITE_GHOSTVAULT_PROXY_URL ||
    process.env.GHOSTVAULT_PROXY_URL ||
    process.env.GHOSTVAULT_BASE_URL ||
    env.VITE_GHOSTVAULT_PROXY_URL ||
    env.GHOSTVAULT_PROXY_URL ||
    env.GHOSTVAULT_BASE_URL ||
    'http://127.0.0.1:8989/api'
  try {
    const u = new URL(raw.includes('://') ? raw : `http://${raw}`)
    const target = `${u.protocol}//${u.host}`
    let basePath = u.pathname.replace(/\/$/, '')
    if (basePath === '/') basePath = ''
    if (basePath) {
      return {
        target,
        rewrite: (path) => basePath + path,
      }
    }
    return { target, rewrite: (path) => path }
  } catch {
    return {
      target: 'http://127.0.0.1:8989',
      rewrite: (path) => '/api' + path,
    }
  }
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, repoEnvDir, ['VITE_', 'GHOSTVAULT_'])
  const v1Proxy = resolveV1Proxy(env)

  const rawBase = env.VITE_DASHBOARD_BASE || '/'
  const base = rawBase === '/' ? '/' : rawBase.endsWith('/') ? rawBase : `${rawBase}/`

  return {
    base,
    envDir: repoEnvDir,
    envPrefix: ['VITE_', 'GHOSTVAULT_'],
    plugins: [react(), tailwindcss()],
    server: {
      host: true,
      port: 5177,
      proxy: {
        '/v1': {
          target: v1Proxy.target,
          changeOrigin: true,
          rewrite: v1Proxy.rewrite,
        },
      },
    },
    build: {
      outDir: 'dist',
    },
  }
})
