# Docker 部署指南

本指南将帮助您使用 Docker 部署 TodoApp 应用。

## 前置要求

- Docker 20.10 或更高版本
- Docker Compose 2.0 或更高版本
- 至少 512MB 可用内存

---

## 快速开始

### 1. 克隆项目并进入目录

```bash
cd todoapp
```

### 2. 配置环境变量

```bash
# 复制环境变量文件
cp .env.docker .env

# 编辑 .env 文件，设置 JWT_SECRET
# 使用以下命令生成安全的随机密钥：
openssl rand -hex 32
```

### 3. 构建并启动

#### 使用 Make（推荐）

```bash
# 构建镜像
make build

# 启动容器
make run

# 查看日志
make logs

# 查看状态
make status
```

#### 使用 Docker Compose

```bash
# 构建并启动
docker-compose up -d --build

# 查看日志
docker-compose logs -f

# 查看状态
docker-compose ps
```

### 4. 访问应用

- **API 端点**: http://localhost:8080/api/v1
- **前端界面**: http://localhost:8080/
- **健康检查**: http://localhost:8080/api/v1/health

---

## Make 命令参考

| 命令 | 说明 |
|------|------|
| `make help` | 显示帮助信息 |
| `make build` | 构建 Docker 镜像 |
| `make run` | 启动容器 |
| `make stop` | 停止容器 |
| `make restart` | 重启容器 |
| `make logs` | 查看容器日志 |
| `make status` | 查看容器状态 |
| `make shell` | 进入容器 shell |
| `make clean` | 清理容器和镜像 |
| `make db-backup` | 备份数据库 |
| `make db-restore` | 恢复数据库 |
| `make health` | 健康检查 |
| `make rebuild` | 重新构建并启动 |

---

## Docker Compose 命令参考

### 基本操作

```bash
# 构建并启动
docker-compose up -d

# 查看日志
docker-compose logs -f todoapp

# 查看状态
docker-compose ps

# 停止容器
docker-compose down

# 重启容器
docker-compose restart

# 重新构建
docker-compose up -d --build
```

### 数据管理

```bash
# 进入数据库
docker-compose exec todoapp sh -c "sqlite3 /app/data/todoapp.db"

# 备份数据库
docker-compose exec -T todoapp sh -c "cat /app/data/todoapp.db" > backup.db

# 恢复数据库
docker-compose exec -T todoapp sh -c "cat > /app/data/todoapp.db" < backup.db
```

### 调试

```bash
# 进入容器 shell
docker-compose exec todoapp sh

# 查看资源使用
docker stats todoapp

# 查看日志（最近 100 行）
docker-compose logs --tail=100 todoapp
```

---

## Dockerfile 说明

### 多阶段构建

Dockerfile 使用多阶段构建来最小化最终镜像体积：

#### 阶段 1: 前端构建器
- 使用 `node:18-alpine` 基础镜像
- 安装依赖并构建 React 应用
- 输出：`/app/web/build` 目录

#### 阶段 2: 后端构建器
- 使用 `golang:1.21-alpine` 基础镜像
- 安装 GCC 和 SQLite 开发库
- 编译 Go 应用（静态链接）
- 输出：`todoapp-server` 可执行文件

#### 阶段 3: 最终镜像
- 使用 `alpine:3.19` 基础镜像（~5MB）
- 只包含运行时依赖
- 复制前端和后端产物
- 最终镜像大小：~50MB

### 优化措施

1. **多阶段构建** - 只保留必要文件
2. **Alpine 基础镜像** - 最小化镜像体积
3. **静态链接** - 减少运行时依赖
4. **UPX 压缩** - 可选的进一步压缩
5. **非 root 用户** - 提高安全性
6. **健康检查** - 自动监控容器状态

---

## 环境变量配置

### 必需变量

| 变量 | 说明 | 示例 |
|------|------|------|
| `JWT_SECRET` | JWT 签名密钥 | `openssl rand -hex 32` |

### 可选变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `ENVIRONMENT` | 运行环境 | `production` |
| `SERVER_PORT` | 服务器端口 | `8080` |
| `DATABASE_URL` | 数据库路径 | `/app/data/todoapp.db` |
| `TZ` | 时区 | `Asia/Shanghai` |
| `MAX_LOGIN_ATTEMPTS` | 最大登录尝试 | `5` |
| `LOGIN_ATTEMPT_WINDOW` | 尝试窗口期 | `15m` |
| `ACCESS_TOKEN_DURATION` | 访问令牌有效期 | `15m` |
| `REFRESH_TOKEN_DURATION` | 刷新令牌有效期 | `168h` |

---

## 持久化存储

Docker Compose 配置使用命名卷来持久化数据库：

```yaml
volumes:
  todoapp-data:
    driver: local
```

数据库存储在容器的 `/app/data/todoapp.db`。

### 备份数据库

```bash
# 使用 Make
make db-backup

# 或手动
docker-compose exec -T todoapp sh -c "cat /app/data/todoapp.db" > backup.db
```

### 恢复数据库

```bash
# 使用 Make
make db-restore

# 或手动
docker-compose exec -T todoapp sh -c "cat > /app/data/todoapp.db" < backup.db
```

---

## 使用 Nginx 反向代理

### 启用 Nginx

```bash
docker-compose --profile with-nginx up -d
```

### 配置说明

Nginx 配置文件：`nginx.conf`

主要功能：
- 静态文件服务
- API 反向代理
- Gzip 压缩
- 健康检查

### SSL/HTTPS 配置

1. 获取 SSL 证书（Let's Encrypt 或其他 CA）

2. 将证书文件放在 `nginx/ssl/` 目录：
   - `cert.pem` - 证书文件
   - `key.pem` - 私钥文件

3. 取消注释 `nginx.conf` 中的 HTTPS server 块

4. 重启容器：
   ```bash
   docker-compose restart
   ```

---

## 生产部署检查清单

### 安全性

- [ ] 设置强 `JWT_SECRET`
- [ ] 使用 HTTPS/TLS
- [ ] 限制网络访问（防火墙）
- [ ] 使用非 root 用户运行
- [ ] 定期更新镜像
- [ ] 配置日志轮转

### 性能

- [ ] 设置资源限制
- [ ] 启用健康检查
- [ ] 配置日志收集
- [ ] 监控资源使用
- [ ] 配置自动重启策略

### 可靠性

- [ ] 配置数据备份
- [ ] 测试恢复流程
- [ ] 设置监控告警
- [ ] 准备灾难恢复计划

---

## 故障排除

### 容器无法启动

```bash
# 查看详细日志
docker-compose logs todoapp

# 检查环境变量
docker-compose config

# 验证配置
docker-compose config --resolve-image-digests
```

### 数据库锁定

```bash
# 重启容器
docker-compose restart todoapp

# 或清理锁文件（谨慎使用）
docker-compose exec todoapp sh -c "rm -f /app/data/todoapp.db-journal"
```

### 健康检查失败

```bash
# 手动检查
curl http://localhost:8080/api/v1/health

# 查看容器状态
docker-compose ps
```

### 内存不足

```bash
# 增加内存限制（编辑 docker-compose.yml）
deploy:
  resources:
    limits:
      memory: 1G
```

### 网络问题

```bash
# 检查网络
docker network ls
docker network inspect todoapp_todoapp-network

# 重新创建网络
docker-compose down
docker-compose up -d
```

---

## 监控和日志

### 查看日志

```bash
# 实时日志
docker-compose logs -f

# 最近 100 行
docker-compose logs --tail=100

# 仅错误日志
docker-compose logs | grep ERROR
```

### 资源监控

```bash
# 实时监控
docker stats todoapp

# 使用 Make
make monitor
```

### 健康检查

```bash
# 使用 Make
make health

# 或手动
curl http://localhost:8080/api/v1/health
```

---

## 更新部署

### 更新镜像

```bash
# 拉取最新代码
git pull

# 重新构建
docker-compose up -d --build

# 或使用 Make
make rebuild
```

### 滚动更新（多容器）

```bash
# 更新一个实例
docker-compose up -d --scale todoapp=2

# 停止旧实例
docker-compose up -d --scale todoapp=1
```

---

## 性能优化

### 减小镜像大小

1. 使用 `.dockerignore` 排除不必要文件
2. 使用多阶段构建
3. 启用 UPX 压缩（在 Dockerfile 中取消注释）

### 提升启动速度

1. 使用缓存层（Docker 自动处理）
2. 预编译依赖（在 Dockerfile 中优化）
3. 使用 `--no-cache` 强制重新构建

### 提升运行性能

1. 增加资源限制
2. 调整数据库连接池
3. 启用 HTTP/2（Nginx）

---

## 安全最佳实践

1. **使用最小权限原则**
   - 非容器使用 root 用户
   - 限制网络访问
   - 使用只读文件系统（如适用）

2. **定期更新**
   ```bash
   docker-compose pull
   docker-compose up -d
   ```

3. **扫描镜像漏洞**
   ```bash
   docker scan todoapp:latest
   ```

4. **使用 secrets 管理敏感数据**
   ```yaml
   secrets:
     jwt_secret:
       file: ./secrets/jwt_secret.txt
   ```

---

## 常见问题

### Q: 镜像大小是多少？

A: 最终镜像大约 50MB，取决于前端构建产物大小。

### Q: 如何自定义端口？

A: 修改 `docker-compose.yml` 中的端口映射：
```yaml
ports:
  - "3000:8080"  # 宿主机端口:容器端口
```

### Q: 如何配置 HTTPS？

A: 参考"使用 Nginx 反向代理"章节。

### Q: 数据存储在哪里？

A: 数据存储在 Docker 命名卷 `todoapp-data` 中。

### Q: 如何备份数据？

A: 使用 `make db-backup` 或手动复制数据库文件。

---

## 相关资源

- [Docker 官方文档](https://docs.docker.com/)
- [Docker Compose 文档](https://docs.docker.com/compose/)
- [Alpine Linux](https://alpinelinux.org/)
- [项目 README](../README.md)
- [部署指南](../DEPLOYMENT.md)

---

**最后更新**: 2026-01-30
**版本**: 1.0.0
