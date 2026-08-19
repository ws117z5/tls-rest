# syntax=docker/dockerfile:1

# =============================================================================
# tls-rest — multi-stage build (with OpenCV + libpcap features enabled)
#   1. frontend : webpack bundle -> js/dist/main.js
#   2. backend  : CGO Go binary, built with -tags "opencv netmap_pcap"
#   3. runtime  : slim image + OpenCV / libpcap runtime libraries
# =============================================================================

# ---- Stage 1: frontend bundle ----------------------------------------------
FROM node:20-bookworm-slim AS frontend
WORKDIR /app/js

# package*.json globs the manifest + committed lockfile; npm ci installs it
# reproducibly. --legacy-peer-deps avoids ERESOLVE from libs that still declare
# pre-React-19 peer ranges.
COPY js/package*.json ./
RUN npm ci --legacy-peer-deps

# Build ONLY the named "prod" webpack config (the config array also has a
# watch:true dev config that never exits in a build).
COPY js/ ./
RUN npx webpack --config-name prod


# ---- Stage 2: Go binary (CGO on) -------------------------------------------
FROM ghcr.io/hybridgroup/opencv:4.13.0 AS backend
ENV GOTOOLCHAIN=auto
RUN apt-get update \
 && apt-get install -y --no-install-recommends libpcap-dev \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -tags "opencv netmap_pcap" \
        -trimpath -ldflags="-s -w" -o /out/tls-rest 


# ---- Stage 3: runtime -------------------------------------------------------
FROM debian:bookworm-slim
# Runtime shared libraries (same Debian 12 => same OpenCV 4.6 ABI as the builder):
#   libopencv-dev pulls every OpenCV runtime module gocv links against — the
#     simplest reliable option. To slim the image, replace it with just the
#     specific libopencv-*406 runtime packages gocv needs.
#   libpcap0.8    -> netmapper capture at runtime
#   ca-certificates -> outbound TLS (OAuth token endpoints, image fetch, …)
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates libpcap0.8 libopencv-dev \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Binary + the assets the server reads from its working directory at runtime.
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
# .env into /app/.private at runtime — nothing cert-related is baked in.
EXPOSE 8443

# Required env (server fails fast without them): PG_ADDR, JWT_SIGNATURE,
# GOOGLE_ID, GOOGLE_SECRET. The compose supplies these via /app/.private/.env.
CMD ["./tls-rest"]