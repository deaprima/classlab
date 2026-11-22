# syntax=docker/dockerfile:1

# ---- build the static app ----
FROM node:26.13.1-alpine AS build
ARG PNPM_VERSION=10.33.2
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@${PNPM_VERSION} --activate
COPY package.json pnpm-lock.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store pnpm install --frozen-lockfile
COPY . .
RUN pnpm build

# ---- serve it: Caddy with the built files; deploy/Caddyfile is mounted over the default ----
FROM caddy:2.11.9-alpine
COPY --from=build /app/dist /srv
