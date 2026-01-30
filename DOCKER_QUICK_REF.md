# Docker 快速参考

## 文件说明

| 文件 | 说明 |
|------|------|
| `Dockerfile` | 多阶段构建配置，包含前端和后端 |
| `docker-compose.yml` | Docker Compose 配置，用于容器编排 |
| `.dockerignore` | Docker 构建时忽略的文件 |
| `.env.docker` | Docker 环境变量模板 |
| `Makefile` | Docker 管理命令集合 |
| `nginx.conf` | Nginx 反向代理配置（可选） |
| `test-docker.sh` | Docker 构建测试脚本 |
| `DOCKER_DEPLOYMENT.md` | 详细部署指南 |

---

## 快速开始

### 1. 初始化

```bash
# 复制环境变量
cp .env.docker .env

# 生成 JWT_SECRET
openssl rand -hex 32
```

### 2. 构建和启动

```bash
# 使用 Make（推荐）
make build
make run

# 或使用 Docker Compose
docker-compose up -d --build
```

### 3. 访问应用

- API: http://localhost:8080/api/v1
- 前端: http://localhost:8080/
- 健康检查: http://localhost:8080/api/v1/health

---

## Make 命令速查

```bash
make build       # 构建镜像
make run         # 启动容器
make stop        # 停止容器
make restart     # 重启容器
make logs        # 查看日志
make status      # 查看状态
make shell       # 进入容器
make clean       # 清理
make db-backup   # 备份数据库
make db-restore  # 恢复数据库
make health      # 健康检查
make rebuild     # 重新构建
make monitor     # 资源监控
```

---

## Docker Compose 命令速查

```bash
# 基本操作
docker-compose up -d              # 后台启动
docker-compose down                # 停止并删除容器
docker-compose restart             # 重启
docker-compose ps                  # 查看状态
docker-compose logs -f             # 查看实时日志

# 构建相关
docker-compose build               # 构建镜像
docker-compose up -d --build       # 重新构建并启动

# 数据管理
docker-compose exec todoapp sh     # 进入容器
docker-compose exec todoapp sh -c "sqlite3 /app/data/todoapp.db"  # 进入数据库

# 清理
docker-compose down -v             # 停止并删除数据卷
docker system prune -f             # 清理未使用的资源
```

---

## 镜像信息

- **基础镜像**: alpine:3.19 (~5MB)
- **最终镜像**: ~50MB
- **前端**: React 18 + TypeScript
- **后端**: Go 1.21 + SQLite

---

## 端口映射

| 容器端口 | 宿主机端口 | 说明 |
|----------|-----------|------|
| 8080 | 8080 | API 和前端 |
| 80 | 80 | Nginx（可选） |
| 443 | 443 | HTTPS（可选） |

---

## 数据持久化

数据库存储在命名卷 `todoapp-data` 中：

```yaml
volumes:
  todoapp-data:
    driver: local
```

### 备份

```bash
docker-compose exec -T todoapp sh -c "cat /app/data/todoapp.db" > backup.db
```

### 恢复

```bash
docker-compose exec -T todoapp sh -c "cat > /app/data/todoapp.db" < backup.db
```

---

## 环境变量

必需变量：
- `JWT_SECRET`: JWT 签名密钥（使用 `openssl rand -hex 32` 生成）

可选变量：
- `ENVIRONMENT`: 运行环境（默认：production）
- `SERVER_PORT`: 服务器端口（默认：8080）
- `DATABASE_URL`: 数据库路径（默认：/app/data/todoapp.db）
- `TZ`: 时区（默认：Asia/Shanghai）

---

## 健康检查

容器包含自动健康检查：

```yaml
healthcheck:
  test: ["CMD", "wget", "--quiet", "--tries=1", "--spider",
         "http://localhost:8080/api/v1/health"]
  interval: 30s
  timeout: 3s
  retries: 3
  start_period: 5s
```

手动检查：

```bash
curl http://localhost:8080/api/v1/health
```

---

## 资源限制

默认配置：

```yaml
deploy:
  resources:
    limits:
      cpus: '1.0'
      memory: 512M
    reservations:
      cpus: '0.25'
      memory: 128M
```

---

## 网络配置

容器连接到桥接网络：

```yaml
networks:
  todoapp-network:
    driver: bridge
```

---

## 日志管理

### 查看日志

```bash
# 所有日志
docker-compose logs

# 实时日志
docker-compose logs -f

# 最近 100 行
docker-compose logs --tail=100

# 仅错误
docker-compose logs | grep ERROR
```

### 日志配置

在 `docker-compose.yml` 中配置：

```yaml
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
```

---

## 安全配置

1. **非 root 用户**: 容器以 `appuser` (uid=1000) 运行
2. **最小化镜像**: 仅包含运行时依赖
3. **只读文件系统**: 可根据需要启用
4. **健康检查**: 自动监控容器状态

---

## 测试

运行测试脚本：

```bash
./test-docker.sh
```

测试脚本会：
1. 检查 Docker 和 Docker Compose
2. 验证必需文件
3. 设置环境变量
4. 构建镜像
5. 启动容器
6. 测试健康检查
7. 测试 API 端点
8. 显示日志

---

## 故障排除

### 容器无法启动

```bash
# 查看日志
docker-compose logs todoapp

# 检查配置
docker-compose config
```

### 数据库锁定

```bash
# 重启容器
docker-compose restart todoapp
```

### 网络问题

```bash
# 重新创建网络
docker-compose down
docker-compose up -d
```

### 端口冲突

修改 `docker-compose.yml` 中的端口映射：

```yaml
ports:
  - "3000:8080"  # 使用宿主机端口 3000
```

---

## 更新和升级

### 更新镜像

```bash
# 拉取最新代码
git pull

# 重新构建
make rebuild

# 或
docker-compose up -d --build
```

### 更新基础镜像

```bash
# 更新 Alpine
docker pull alpine:3.19

# 重新构建
make rebuild
```

---

## 监控

### 查看资源使用

```bash
# 实时监控
docker stats todoapp

# 使用 Make
make monitor
```

### 容器状态

```bash
# 查看状态
make status

# 或
docker-compose ps
```

---

## 性能优化

1. **多阶段构建**: 已启用
2. **Alpine 基础镜像**: 已使用
3. **静态链接**: 已配置
4. **UPX 压缩**: 可选（在 Dockerfile 中取消注释）
5. **资源限制**: 已配置

---

## 生产部署清单

- [ ] 设置强 `JWT_SECRET`
- [ ] 配置 HTTPS/TLS
- [ ] 设置防火墙规则
- [ ] 配置日志轮转
- [ ] 设置监控告警
- [ ] 配置自动备份
- [ ] 测试恢复流程
- [ ] 配置资源限制
- [ ] 设置自动重启策略
- [ ] 定期更新镜像

---

## 相关文档

- [详细部署指南](DOCKER_DEPLOYMENT.md)
- [部署指南](DEPLOYMENT.md)
- [项目 README](README.md)

---

**最后更新**: 2026-01-30
**版本**: 1.0.0
