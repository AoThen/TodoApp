# 构建阶段 1: 前端 (React)
FROM node:18-alpine AS frontend-builder

WORKDIR /app/web

# 复制前端依赖文件
COPY web/package*.json ./

# 安装依赖（使用 npm install，因为没有 package-lock.json）
RUN npm install --omit=dev && npm cache clean --force

# 复制前端源代码
COPY web/ ./

# 构建前端（生产模式）
RUN npm run build

# 构建阶段 2: 后端 (Go)
FROM golang:1.21-alpine AS backend-builder

# 安装必要的构建工具
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# 复制 go 依赖文件
COPY go.mod go.sum ./

# 下载依赖（利用 Docker 缓存）
RUN go mod download

# 复制源代码
COPY . .

# 构建后端（静态链接，使用 upx 压缩）
RUN CGO_ENABLED=1 go build \
    -ldflags="-s -w -linkmode external -extldflags '-static'" \
    -o todoapp-server \
    .

# 可选：使用 upx 压缩可执行文件
# RUN apk add --no-cache upx && upx --best --lzma todoapp-server

# 最终阶段：运行时镜像
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    sqlite-libs

# 创建非 root 用户
RUN addgroup -g 1000 appgroup && \
    adduser -D -u 1000 -G appgroup appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制后端可执行文件
COPY --from=backend-builder --chown=appuser:appgroup /app/todoapp-server .

# 从构建阶段复制前端静态文件
COPY --from=frontend-builder --chown=appuser:appgroup /app/web/build ./web/build

# 复制环境变量模板
COPY --chown=appuser:appgroup .env.example .env.example

# 创建数据库目录
RUN mkdir -p /app/data && \
    chown -R appuser:appgroup /app/data

# 设置时区（可选，根据需要修改）
ENV TZ=Asia/Shanghai

# 切换到非 root 用户
USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

# 设置环境变量
ENV ENVIRONMENT=production
ENV DATABASE_URL=/app/data/todoapp.db

# 启动命令
CMD ["./todoapp-server"]
