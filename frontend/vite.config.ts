import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  // Base URL pour le déploiement côte-à-côte avec le legacy : la SPA est
  // montée sous /shop2/ par le backend Go.
  base: '/shop2/',
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
