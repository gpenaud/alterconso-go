# ─── Stage 1 : dépendances ───────────────────────────────────────────────────

FROM golang:1.22-alpine AS deps

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

# ─── Stage 2 : build ─────────────────────────────────────────────────────────

FROM deps AS builder

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
    -o alterconso ./cmd/server

# ─── Stage 3 : image finale ─────────────────────────────────────────────────── 
FROM alpine:3.20

LABEL org.opencontainers.image.title="alterconso" \
      org.opencontainers.image.description="Alterconso – gestion de groupements d'achat (AMAP/CSA)" \
      org.opencontainers.image.source="https://github.com/gpenaud/alterconso-go" \
      org.opencontainers.image.licenses="AGPL-3.0"

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser  -S app -G app

WORKDIR /app

COPY --from=builder /app/alterconso .

ENV PORT=8080 \
    TZ=Europe/Paris

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:${PORT}/livez || exit 1

USER app

ENTRYPOINT ["/app/alterconso"]
