# syntax=docker/dockerfile:1.7
#
# alterconso — image Docker
#
# Build :
#   DOCKER_BUILDKIT=1 docker build \
#     --build-arg VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo dev) \
#     -t alterconso:latest .
#
# Run (recommandé pour la sécurité — read-only FS, no caps, non-root) :
#   docker run --rm -p 8080:8080 \
#     --read-only --tmpfs /tmp \
#     --cap-drop=ALL --security-opt=no-new-privileges \
#     -v $PWD/config.yaml:/app/config.yaml:ro \
#     -e DB_PASSWORD=... -e JWT_SECRET=... -e APP_KEY=... \
#     alterconso:latest

ARG GO_VERSION=1.23
ARG ALPINE_VERSION=3.20

# ─── Stage 1 : build ─────────────────────────────────────────────────────────
FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder

# tzdata pour copier zoneinfo dans la stage runtime distroless,
# brotli pour pré-compresser les assets statiques (gzip est dans busybox).
RUN apk add --no-cache tzdata brotli

WORKDIR /src

# Dépendances en couche séparée : recompilation plus rapide quand seul
# le code source change.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download -x

# Source nécessaire au build (whitelist explicite — le reste du repo ne
# rentre jamais dans l'image, même si .dockerignore l'oublie).
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

# Pré-compression des assets texte (JS, CSS, SVG, JSON, HTML, TXT).
# On crée <fichier>.br ET <fichier>.gz, puis on supprime l'original :
# l'image ne porte que les versions compressées, le handler négocie via
# Accept-Encoding (br > gzip > 404). Économise ~70 % sur libs.prod.js & co.
COPY www /assets/www
RUN find /assets/www -type f \( \
        -name '*.js' -o -name '*.css' -o -name '*.svg' -o \
        -name '*.html' -o -name '*.txt' -o -name '*.json' \
      \) -size +1k \
      -exec sh -c 'brotli -q 11 -- "$1" && gzip -9 -- "$1"' _ {} \;

# ─── Stage 2 : runtime distroless ───────────────────────────────────────────
# distroless/static : pas de shell, pas de package manager, pas de libc.
# Inclut déjà /etc/ssl/certs/ca-certificates.crt et /etc/{passwd,group}.
# UID nonroot = 65532 par convention.
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="alterconso" \
      org.opencontainers.image.description="Alterconso – gestion de groupements d'achat (AMAP/CSA)" \
      org.opencontainers.image.source="https://github.com/gpenaud/alterconso-go" \
      org.opencontainers.image.licenses="AGPL-3.0"

# Données zoneinfo pour TZ / time.LoadLocation.
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

WORKDIR /app

# Binaire + assets runtime (templates HTML et fichiers statiques).
# www/ vient de la stage builder où les assets texte ont été pré-compressés.
# config.yaml volontairement absent : à monter en read-only au démarrage.
COPY --from=builder --chown=nonroot:nonroot /out/alterconso /app/alterconso
COPY --chown=nonroot:nonroot templates ./templates
COPY --from=builder --chown=nonroot:nonroot /assets/www ./www

USER nonroot:nonroot

ENV PORT=8080 \
    TZ=Europe/Paris

EXPOSE 8080

# Pas de HEALTHCHECK Dockerfile : distroless n'a ni shell, ni wget, ni curl.
# Healthcheck à brancher côté orchestrateur :
#   K8s livenessProbe / readinessProbe → httpGet path=/livez port=8080
#   docker run             → utiliser un sidecar ou un check externe

ENTRYPOINT ["/app/alterconso"]
