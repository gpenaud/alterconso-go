# syntax=docker/dockerfile:1.7
#
# alterconso — image Docker
#
# Build :
#   docker build \
#     --build-arg VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo dev) \
#     -t alterconso:latest .
#
# Run minimal :
#   docker run --rm -p 8080:8080 \
#     -v $PWD/config.yaml:/app/config.yaml:ro \
#     -e DB_PASSWORD=... -e JWT_SECRET=... -e APP_KEY=... \
#     alterconso:latest
#
# Run hardenné (recommandé en prod) :
#   docker run --rm -p 8080:8080 \
#     --read-only --tmpfs /tmp \
#     --cap-drop=ALL --security-opt=no-new-privileges \
#     -v $PWD/config.yaml:/app/config.yaml:ro \
#     alterconso:latest

ARG GO_VERSION=1.23
ARG NODE_VERSION=22
ARG ALPINE_VERSION=3.20

# ─── Stage 1 : frontend (Vite build) ──────────────────────────────────────────
FROM node:${NODE_VERSION}-alpine${ALPINE_VERSION} AS frontend

WORKDIR /src

# Lockfile en couche séparée : npm ci ne refait pas l'install si seules
# les sources TS/TSX changent.
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --no-fund

COPY frontend/ ./
RUN npm run build


# ─── Stage 2 : backend Go ─────────────────────────────────────────────────────
FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder

# tzdata pour copier zoneinfo dans la stage runtime distroless,
# brotli pour pré-compresser les assets statiques (gzip est dans busybox).
RUN apk add --no-cache tzdata brotli

WORKDIR /src

# Dépendances en couche séparée — recompile rapide quand seul le code change.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download -x

# Whitelist explicite des sources : rien d'autre n'entre dans l'image.
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
COPY docs ./docs

ARG VERSION=dev

# Build statique, minimal, reproductible :
#   -trimpath  : retire les chemins absolus du binaire
#   -s -w      : strip table de symboles + DWARF
#   -buildid=  : zéro l'ID de build (reproductible bit-à-bit)
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build \
        -trimpath \
        -ldflags="-s -w -buildid= -X main.version=${VERSION}" \
        -o /out/alterconso \
        ./cmd/server

# Pré-compression des assets texte legacy (www/css, www/js, www/font, etc.).
# On crée <fichier>.br ET <fichier>.gz, puis on supprime l'original :
# l'image ne porte que les versions compressées, le handler négocie via
# Accept-Encoding (br > gzip > 404). Économise ~70 % sur les blobs.
COPY www /assets/www
RUN find /assets/www -type f \( \
        -name '*.js' -o -name '*.css' -o -name '*.svg' -o \
        -name '*.html' -o -name '*.txt' -o -name '*.json' \
      \) -size +1k \
      -exec sh -c 'brotli -q 11 -- "$1" && gzip -9 -- "$1"' _ {} \;


# ─── Stage 3 : runtime distroless ─────────────────────────────────────────────
# distroless/static : pas de shell, pas de package manager, pas de libc.
# Inclut /etc/ssl/certs/ca-certificates.crt et /etc/{passwd,group}.
# UID nonroot = 65532 par convention.
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="alterconso" \
      org.opencontainers.image.description="Alterconso – gestion de groupements d'achat (AMAP/CSA)" \
      org.opencontainers.image.source="https://github.com/gpenaud/alterconso-go" \
      org.opencontainers.image.licenses="AGPL-3.0"

# Données zoneinfo pour TZ / time.LoadLocation.
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

WORKDIR /app

# Binaire Go.
COPY --from=builder --chown=nonroot:nonroot /out/alterconso /app/alterconso

# Templates Go HTML (rendu serveur des pages legacy).
COPY --chown=nonroot:nonroot templates ./templates

# Assets legacy (www/) avec les .br/.gz pré-compressés.
COPY --from=builder --chown=nonroot:nonroot /assets/www ./www

# Bundle React (frontend/dist) servi par r.Static("/assets", ...) et
# l'index.html par le NoRoute fallback pour les routes SPA.
COPY --from=frontend --chown=nonroot:nonroot /src/dist ./frontend/dist

USER nonroot:nonroot

ENV PORT=8080 \
    TZ=Europe/Paris

EXPOSE 8080

# Pas de HEALTHCHECK Dockerfile : distroless n'a ni shell, ni curl.
# À brancher côté orchestrateur :
#   K8s :  livenessProbe / readinessProbe → httpGet path=/livez port=8080

ENTRYPOINT ["/app/alterconso"]
