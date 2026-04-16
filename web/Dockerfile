# syntax=docker/dockerfile:1.7
FROM node:22-bookworm-slim AS builder

ARG NPM_REGISTRY=https://registry.npmmirror.com/
ARG VITE_WS_URL=
ARG VITE_API_URL=

ENV VITE_WS_URL=${VITE_WS_URL} \
    VITE_API_URL=${VITE_API_URL} \
    NPM_CONFIG_REGISTRY=${NPM_REGISTRY}

WORKDIR /app/web

COPY web/package*.json ./
COPY web/.npmrc ./
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    npm config set registry "${NPM_REGISTRY}" && \
    npm ci --include=optional

COPY web/ ./
RUN npm run build

FROM nginx:1.28-alpine AS runner

ARG APK_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/alpine

RUN sed -i "s#https://dl-cdn.alpinelinux.org/alpine#${APK_MIRROR}#g" /etc/apk/repositories

COPY deploy/nginx.conf /etc/nginx/nginx.conf
COPY --from=builder /app/web/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
