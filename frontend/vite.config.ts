import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  // SPA montée à la racine côté Go : les routes SPA (/login, /groups/...,
  // /shop/:id, /profile) sont servies par index.html via NoRoute, et /assets
  // par r.Static. Pas de base.
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
      // Assets statiques servis par Go (réutilisés tels quels par le shop React).
      '/img': 'http://localhost:8080',
      '/font': 'http://localhost:8080',
      '/file': 'http://localhost:8080',
      '/locales': 'http://localhost:8080',
    },
  },
})
