FROM node:22-alpine AS builder

ARG APK_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/alpine
ARG NPM_REGISTRY=https://registry.npmmirror.com/
ARG VITE_WS_URL=
ARG VITE_API_URL=
ARG VITE_DEFAULT_MODEL=glm-5

ENV VITE_WS_URL=${VITE_WS_URL} \
    VITE_API_URL=${VITE_API_URL} \
    VITE_DEFAULT_MODEL=${VITE_DEFAULT_MODEL} \
    NPM_CONFIG_REGISTRY=${NPM_REGISTRY}

WORKDIR /app/web

RUN sed -i "s#https://dl-cdn.alpinelinux.org/alpine#${APK_MIRROR}#g" /etc/apk/repositories

COPY web/package*.json ./
COPY web/.npmrc ./
RUN npm config set registry "${NPM_REGISTRY}"
RUN npm ci

COPY web/ ./
RUN npm run build

FROM nginx:1.28-alpine AS runner

ARG APK_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/alpine

RUN sed -i "s#https://dl-cdn.alpinelinux.org/alpine#${APK_MIRROR}#g" /etc/apk/repositories

COPY deploy/nginx.conf /etc/nginx/nginx.conf
COPY --from=builder /app/web/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
