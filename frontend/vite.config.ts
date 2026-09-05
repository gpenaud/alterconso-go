import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  // SPA montée à la racine côté Go : les routes SPA (/login, /groups/...,
  // /shop/:id, /profile) sont servies par index.html via NoRoute, et /assets
  // par r.Static. Pas de base.
  plugins: [react(), tailwindcss()],
  server: {
    // Port fixe, et echec bruyant s il est pris. Par defaut Vite glisse
    // silencieusement au port suivant : 5173 etant occupe par un devspace
    // d un autre projet, le serveur s est retrouve sur 5174 sans que rien ne
    // le dise, et l adresse documentee ne repondait plus rien du tout.
    port: 5174,
    strictPort: true,
    proxy: {
      '/api': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
      // Assets statiques servis par Go (réutilisés tels quels par le shop React).
      '/img': 'http://localhost:8080',
      // La barre finale compte : sans elle, ce prefixe capture aussi « /fonts/ »,
      // ou vivent les polices de la refonte, qui partaient alors vers Go et
      // revenaient en 404 — l interface s affichait en police de repli sans que
      // rien ne le signale.
      '/font/': 'http://localhost:8080',
      // Pages Go de la session (login, choix du groupe, deconnexion). En
      // production Go sert tout sous la meme origine ; sans ce proxy, un lien
      // vers /user/choose tombe sur Vite, qui renvoie la SPA et donc du vide.
      //
      // Barre finale, meme raison que pour /font/ : ce prefixe ne doit capturer
      // que ses propres pages.
      '/user/': 'http://localhost:8080',
      '/file': 'http://localhost:8080',
      '/locales': 'http://localhost:8080',
    },
  },
})
