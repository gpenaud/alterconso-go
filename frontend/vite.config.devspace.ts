import { defineConfig, mergeConfig } from 'vite'
import base from './vite.config'

// Surcouche de vite.config.ts pour l'exécution dans le cluster (devspace.yaml).
// Le fichier de base reste la référence — plugins et proxy /api y sont définis
// une seule fois, et ne sont pas redits ici.
//
// Ces trois réglages n'ont pas d'équivalent en ligne de commande, d'où ce
// fichier plutôt que des drapeaux passés à `vite`.
export default mergeConfig(
  base,
  defineConfig({
    server: {
      // Dans un conteneur, Vite n'écoute que sur la loopback par défaut : le
      // Service ne pourrait pas l'atteindre.
      host: '0.0.0.0',
      port: 5173,

      // Vite 6+ rejette les requêtes dont l'en-tête Host lui est inconnu
      // (protection contre le DNS rebinding). Sans cette ligne, l'Ingress
      // reçoit une page « Blocked request ».
      allowedHosts: ['alterconso.localhost'],

      hmr: {
        // Le client HMR déduit son URL de la page servie. Derrière l'Ingress,
        // la page est en https sur 443, alors que Vite écoute en clair sur
        // 5173 : sans ces valeurs, le navigateur tenterait ws://…:5173 et le
        // rechargement à chaud resterait muet.
        protocol: 'wss',
        host: 'alterconso.localhost',
        clientPort: 443,
      },

      watch: {
        // Les écritures passent par la synchronisation DevSpace, que la
        // surveillance d'événements du noyau ne voit pas toujours dans un
        // conteneur. Le scrutin est plus coûteux mais fiable.
        usePolling: true,
        interval: 300,
      },
    },
  }),
)
