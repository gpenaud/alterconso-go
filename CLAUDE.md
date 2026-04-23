# alterconso-go

Application de gestion de groupements d'achat AMAP/CSA. Réécriture en Go de l'application originale PHP (dans `~/work/alterconso/`).

## Stack

**Backend** : Go 1.23, Gin, GORM, MySQL 8  
**Frontend** : React 19, TypeScript, Vite, Tailwind CSS 4, React Query, Zustand  
**Auth** : JWT (golang-jwt)

## Structure

```
cmd/server/main.go          # point d'entrée
internal/
  config/                   # chargement de la config (.env)
  db/                       # connexion et migrations GORM
  handler/                  # handlers Gin (API + pages HTML legacy)
  middleware/                # CORS, auth JWT
  model/                    # modèles GORM
  service/                  # logique métier, cron
frontend/src/
  api/                      # clients axios par domaine
  pages/                    # pages React
  components/               # composants partagés
  store/                    # état global Zustand
templates/                  # anciens templates Go HTML (legacy, ne pas modifier)
```

## Commandes

```bash
# Backend
docker-compose up              # démarre app + MySQL
go run ./cmd/server            # sans Docker

# Frontend
cd frontend
npm install
npm run dev                    # dev server Vite (port 5173)
npm run build                  # build prod dans frontend/dist
```

## Travail en cours

L'interface React (`frontend/`) est en cours de développement — c'est la priorité. Les anciens templates Go HTML (`templates/`) sont l'interface de référence fonctionnelle (ancienne app PHP dans `~/work/alterconso/`).
