# 多阶段构建
# 第一阶段：构建静态资源
FROM node:25-alpine AS builder

ARG VITE_WS_URL=ws://localhost/agent/v1/chat/ws
ARG VITE_API_URL=http://localhost/agent/v1
ARG VITE_DEFAULT_MODEL=glm-5

ENV VITE_WS_URL=${VITE_WS_URL} \
    VITE_API_URL=${VITE_API_URL} \
    VITE_DEFAULT_MODEL=${VITE_DEFAULT_MODEL}

WORKDIR /app

COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

# 第二阶段：使用 nginx 提供静态站点
FROM nginx:alpine AS runner

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 3000

CMD ["nginx", "-g", "daemon off;"]
