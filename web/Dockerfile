# syntax=docker/dockerfile:1.7
FROM node:22-bookworm-slim AS builder

ARG PNPM_REGISTRY=https://registry.npmmirror.com/
ARG PNPM_VERSION=9.15.2
ARG VITE_WS_URL=
ARG VITE_API_URL=

ENV VITE_WS_URL=${VITE_WS_URL} \
    VITE_API_URL=${VITE_API_URL} \
    COREPACK_NPM_REGISTRY=${PNPM_REGISTRY} \
    PNPM_HOME=/pnpm \
    PATH=/pnpm:${PATH}

WORKDIR /app/web

COPY web/package.json web/pnpm-lock.yaml ./
COPY web/.npmrc ./
RUN --mount=type=cache,target=/pnpm/store,sharing=locked \
    corepack enable && \
    corepack prepare "pnpm@${PNPM_VERSION}" --activate && \
    pnpm config set registry "${PNPM_REGISTRY}" && \
    pnpm config set store-dir /pnpm/store && \
    pnpm install --frozen-lockfile

COPY web/ ./
RUN pnpm run build

FROM nginx:1.28-alpine AS runner

ARG APK_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/alpine

RUN sed -i "s#https://dl-cdn.alpinelinux.org/alpine#${APK_MIRROR}#g" /etc/apk/repositories

COPY deploy/nginx.conf /etc/nginx/nginx.conf
COPY --from=builder /app/web/dist /usr/share/nginx/html

EXPOSE 80
EXPOSE 443

CMD ["nginx", "-g", "daemon off;"]
