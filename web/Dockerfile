# 多阶段构建
# 第一阶段：构建阶段
FROM node:25-alpine AS builder

# 构建时环境变量 - 这些会被 docker-compose 中的环境变量覆盖
ENV NEXT_PUBLIC_WS_URL=ws://localhost/agent/v1/chat/ws \
    NEXT_PUBLIC_API_URL=http://localhost/agent/v1 \
    NEXT_PUBLIC_DEFAULT_MODEL=glm-5

# 设置工作目录
WORKDIR /app

# 复制 package.json 和 package-lock.json（如果存在）
COPY package*.json ./

# 安装依赖
RUN npm ci

# 复制源代码
COPY . .

# 构建应用
RUN npm run build

# 第二阶段：运行阶段
FROM node:25-alpine AS runner

# 设置工作目录
WORKDIR /app

# 创建非 root 用户
RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

# 复制构建产物
COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

# 设置用户
USER nextjs

# 暴露端口
EXPOSE 3000

ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

# 启动应用
CMD ["node", "server.js"]