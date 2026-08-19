# syntax=docker/dockerfile:1

# =============================================================================
# tls-rest — multi-stage build
#   1. frontend : webpack bundle  -> js/dist/main.js
#   2. backend  : static Go binary
#   3. runtime  : minimal image with the binary + served assets
# =============================================================================

# ---- Stage 1: frontend bundle ----------------------------------------------
FROM node:20-bookworm-slim AS frontend
WORKDIR /app/js

# Install against the lockfile first for good layer caching.
COPY js/package.json js/package-lock.json ./
RUN npm ci

# Build ONLY the named "prod" webpack config. The exported config array also
# contains a dev config with `watch: true`, which would never exit inside a
# build; --config-name selects just the one-shot production build.
COPY js/ ./
RUN npx webpack --config-name prod


# ---- Stage 2: Go binary -----------------------------------------------------
FROM golang:1.25-bookworm AS backend
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Default build tags only. The optional netmapper packet-capture path
# (-tags netmap_pcap) needs libpcap; leaving it off keeps this a static,
# dependency-free binary, so CGO can be disabled.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/tls-rest .


# ---- Stage 3: runtime -------------------------------------------------------
FROM debian:bookworm-slim
# ca-certificates: outbound TLS (OAuth token endpoints, image fetch, …).
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Binary + the assets the server reads from its working directory at runtime:
#   go.config.json  – module manifest (read on startup)
#   templates/      – index.gohtml / error.gohtml (SPA shell)
#   css/ img/       – static assets served at /css/ and /img/
#   js/dist/        – the webpack bundle served at /js/dist/main.js
#   init/           – SQL migrations
COPY --from=backend  /out/tls-rest      ./tls-rest
COPY go.config.json                      ./go.config.json
COPY templates                           ./templates
COPY css                                 ./css
COPY img                                 ./img
#COPY init                                ./init
COPY --from=frontend /app/js/dist        ./js/dist

# The server writes application logs here.
RUN mkdir -p logs

# HTTPS only: the server listens on :8443 (see server.go) and loads its cert from
# .private/cert.pem + .private/key.pem. The compose bind-mounts the cert, key and
# .env into /app/.private at runtime — nothing cert-related is baked into the image.
EXPOSE 8443

# Required env (the server fails fast without them):
#   PG_ADDR, JWT_SIGNATURE, GOOGLE_ID, GOOGLE_SECRET
# Common optional env:
#   APP_HOSTS, PG_HOST, PG_PORT, PG_SSLMODE, REDIS_ADDR, MONGO_ADDR,
#   GITHUB_ID/SECRET, FACEBOOK_ID/SECRET, VK_ID/VK_SECRET_KEY
# The compose mounts these via /app/.private/.env; see .env.example.

CMD ["./tls-rest"]