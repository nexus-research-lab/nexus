# syntax=docker/dockerfile:1.7
FROM node:22-bookworm-slim AS builder

ARG PNPM_REGISTRY=https://registry.npmjs.org/
ARG PNPM_VERSION=9.15.2
ARG VITE_WS_URL=
ARG VITE_API_URL=

ENV VITE_WS_URL=${VITE_WS_URL} \
    VITE_API_URL=${VITE_API_URL} \
    PNPM_HOME=/pnpm \
    PATH=/pnpm:${PATH}

WORKDIR /app/web

COPY web/package.json web/pnpm-lock.yaml ./
COPY web/.npmrc ./
RUN --mount=type=cache,target=/pnpm/store,sharing=locked \
    npm install --global --no-audit --no-fund "pnpm@${PNPM_VERSION}" --registry "${PNPM_REGISTRY}" && \
    pnpm config set registry "${PNPM_REGISTRY}" && \
    pnpm config set store-dir /pnpm/store && \
    pnpm install --frozen-lockfile

COPY web/ ./
COPY docs/specs/room-collaboration-mechanism.md /app/docs/specs/room-collaboration-mechanism.md
RUN pnpm run build

FROM nginx:1.28-alpine AS runner

COPY deploy/nginx.conf /etc/nginx/nginx.conf
COPY --from=builder /app/web/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
