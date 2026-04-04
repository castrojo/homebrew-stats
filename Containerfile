# syntax=docker/dockerfile:1
# Multi-stage Chainguard build — Renovate updates all digests automatically.
#
# Stage 1: Build Go binary and generate stats.json
# Stage 2: Build Astro static site
# Stage 3: Serve with Chainguard nginx (nonroot, port 8080)

ARG SKIP_GO_SYNC=false

# ── Stage 1: Go pipeline ────────────────────────────────────────────────────
FROM cgr.dev/chainguard/go:latest AS go-builder

ARG SKIP_GO_SYNC=false

WORKDIR /build
COPY stats-go/ ./stats-go/

RUN cd stats-go && go build -o stats ./cmd/stats/

RUN mkdir -p src/data
COPY src/data/ ./src/data/

# Run the sync unless SKIP_GO_SYNC=true (used in CI where data is pre-generated).
RUN if [ "${SKIP_GO_SYNC}" != "true" ]; then cd stats-go && ./stats; fi

# ── Stage 2: Astro site builder ─────────────────────────────────────────────
FROM cgr.dev/chainguard/node:latest-dev AS site-builder

USER root
WORKDIR /build

COPY package.json package-lock.json* ./
RUN npm ci

COPY src/ ./src/
COPY public/ ./public/
COPY astro.config.mjs tsconfig.json ./

COPY --from=go-builder /build/src/data/ ./src/data/

RUN npm run build

# ── Stage 3: Runtime ─────────────────────────────────────────────────────────
FROM cgr.dev/chainguard/nginx:latest

COPY --from=site-builder /build/dist/ /usr/share/nginx/html/bootc-ecosystem/
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 8080
