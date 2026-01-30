# 部署和运行指南

## 环境要求

### 后端 (Go)
- Go 1.19 或更高版本
- GCC 编译器（用于 SQLite CGO 支持）
- SQLite3

### 前端 (React/TypeScript)
- Node.js 18 或更高版本
- npm 或 yarn

### Android
- Android Studio 或 Android SDK
- JDK 17
- Gradle

---

## 快速开始

### 1. 环境变量设置

```bash
# 复制环境变量模板
cp .env.example .env

# 编辑 .env 文件，必须设置 JWT_SECRET
# 使用以下命令生成安全的随机密钥：
openssl rand -hex 32
```

### 2. 后端部署

#### Ubuntu/Debian (推荐)
```bash
# 安装 GCC（SQLite 需要）
sudo apt-get update
sudo apt-get install gcc

# 设置环境变量
export JWT_SECRET="your-secure-secret-here"
export ENVIRONMENT=development

# 运行开发服务器
go run main.go

# 或构建后运行
CGO_ENABLED=1 go build -o todoapp-server .
./todoapp-server
```

#### macOS
```bash
# macOS 通常已包含 GCC
export JWT_SECRET="your-secure-secret-here"
export ENVIRONMENT=development

go run main.go
```

#### 生产部署
```bash
# 构建生产版本
CGO_ENABLED=1 go build -ldflags="-s -w" -o todoapp-server .

# 使用 systemd 管理（推荐）
sudo cp todoapp.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable todoapp
sudo systemctl start todoapp
```

#### Docker 部署
```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY . .
RUN CGO_ENABLED=1 go build -o todoapp-server .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/todoapp-server .
EXPOSE 8080
CMD ["./todoapp-server"]
```

### 3. 前端部署

#### 开发环境
```bash
cd web

# 安装依赖
npm install

# 设置 API URL（可选）
echo "REACT_APP_API_URL=http://localhost:8080/api/v1" > .env

# 启动开发服务器
npm start
```

#### 生产构建
```bash
cd web

# 安装依赖（包括分析工具）
npm install
npm install --save-dev source-map-explorer

# 构建
npm run build

# 分析包大小
npm run analyze
```

#### 部署到 Nginx
```nginx
server {
    listen 80;
    server_name todoapp.example.com;

    root /var/www/todoapp/build;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    # API 反向代理
    location /api/ {
        proxy_pass http://localhost:8080/api/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

### 4. Android 部署

#### 开发调试
```bash
cd android

# 连接设备或启动模拟器
adb devices

# 安装调试版本
./gradlew installDebug

# 查看日志
adb logcat | grep todoapp
```

#### 生产构建
```bash
cd android

# 构建 Release APK
./gradlew assembleRelease

# 生成的 APK 位置
ls -lh app/build/outputs/apk/release/

# 生成签名 APK（如果已配置）
./gradlew assembleRelease
```

---

## 配置说明

### 后端配置

#### 数据库
默认使用 SQLite 文件 `todoapp.db`。可以修改：
```go
// main.go
db.InitDB("todoapp.db")
```

#### 端口
默认端口 8080。可以通过环境变量修改：
```bash
export SERVER_PORT=3000
```

#### 安全设置
- `JWT_SECRET`: 必须，至少 32 字符
- `ENVIRONMENT`: development 或 production
- `MAX_LOGIN_ATTEMPTS`: 最大登录尝试次数（默认 5）
- `LOGIN_ATTEMPT_WINDOW`: 尝试窗口期（默认 15m）

### 前端配置

#### API 端点
```bash
# web/.env
REACT_APP_API_URL=http://localhost:8080/api/v1
```

#### 超时设置
默认 30 秒。可在 `api.ts` 中修改：
```typescript
const REQUEST_TIMEOUT = 30000;
```

### Android 配置

#### API 端点
修改 `RetrofitClient.kt`:
```kotlin
private const val BASE_URL = "http://10.0.2.2:8080/api/v1/"
// 生产环境使用真实 IP
```

#### 网络安全
`network_security_config.xml` 中配置：
```xml
<domain-config cleartextTrafficPermitted="true">
    <domain includeSubdomains="true">localhost</domain>
</domain-config>
```

---

## 常见问题

### 后端问题

**Q: 编译错误 "cgo: C compiler not found"**

A: 需要安装 GCC 编译器：
```bash
# Ubuntu/Debian
sudo apt-get install gcc

# CentOS/RHEL
sudo yum install gcc

# macOS
xcode-select --install
```

**Q: SQLite 错误 "database is locked"**

A: 已启用 WAL 模式，但仍可能发生。确保只有一个服务器实例运行。

**Q: JWT 密钥错误**

A: 确保设置了 JWT_SECRET 环境变量，且长度至少 32 字符。

### 前端问题

**Q: API 请求失败**

A: 检查：
1. 后端服务器是否运行
2. CORS 配置是否正确
3. API URL 是否正确
4. 令牌是否有效

**Q: 构建失败**

A: 清除缓存重试：
```bash
rm -rf node_modules package-lock.json
npm install
npm run build
```

### Android 问题

**Q: 无法连接到本地服务器**

A: Android 模拟器中使用 `10.0.2.2` 而非 `localhost`。

**Q: 构建失败 "SDK location not found"**

A: 在 `local.properties` 中设置 SDK 路径：
```properties
sdk.dir=/path/to/android/sdk
```

---

## 监控和日志

### 后端日志
```bash
# 查看实时日志
journalctl -u todoapp -f

# 查看错误日志
journalctl -u todoapp -p err
```

### 数据库维护
```bash
# 清理过期令牌（可通过 API 调用）
# 或手动执行：
sqlite3 todoapp.db "DELETE FROM tokens WHERE expires_at < datetime('now');"

# 清理旧登录日志
sqlite3 todoapp.db "DELETE FROM login_logs WHERE timestamp < datetime('now', '-30 days');"
```

### 性能监控
```bash
# 检查进程
ps aux | grep todoapp-server

# 检查内存使用
top -p $(pidof todoapp-server)

# 检查连接数
netstat -an | grep :8080 | wc -l
```

---

## 备份和恢复

### 数据库备份
```bash
# 备份
cp todoapp.db todoapp.db.backup.$(date +%Y%m%d)

# 或使用 sqlite3 导出
sqlite3 todoapp.db .dump > backup.sql
```

### 数据库恢复
```bash
# 从备份恢复
cp todoapp.db.backup.20240130 todoapp.db

# 从 SQL 导入
sqlite3 todoapp.db < backup.sql
```

---

## 生产环境检查清单

部署到生产环境前，确保：

- [ ] 设置 `ENVIRONMENT=production`
- [ ] 配置强 `JWT_SECRET`（使用 `openssl rand -hex 32` 生成）
- [ ] 启用 HTTPS/TLS
- [ ] 使用强数据库密码（如使用 PostgreSQL）
- [ ] 配置防火墙规则
- [ ] 设置日志轮转
- [ ] 配置 CORS 只允许生产域名
- [ ] 启用监控和警报
- [ ] 实施备份策略
- [ ] 配置 DDoS 保护
- [ ] 运行所有测试
- [ ] 进行安全审计
- [ ] 配置速率限制（已在代码中实现）

---

## 性能调优建议

### 后端
- 使用 PostgreSQL 替代 SQLite（高并发场景）
- 启用 GZIP 压缩
- 配置 CDN 静态资源
- 实施缓存层（Redis）
- 使用反向代理（Nginx）

### 前端
- 启用 Service Worker 缓存
- 使用 CDN 分发静态资源
- 启用 Brotli 压缩
- 实施代码分割
- 使用图片懒加载

### Android
- 使用 ProGuard 混淆
- 启用 APK 分片
- 优化图片资源
- 使用网络缓存

---

## 安全最佳实践

1. **定期更新依赖**
   ```bash
   go get -u ./...
   npm update
   ```

2. **使用 HTTPS**
   ```nginx
   ssl_certificate /path/to/cert.pem;
   ssl_certificate_key /path/to/key.pem;
   ```

3. **限制数据库访问**
   ```bash
   chmod 600 todoapp.db
   ```

4. **定期备份数据**
   ```bash
   # 设置 cron 任务
   0 2 * * * cp /path/to/todoapp.db /backup/todoapp.db.$(date +\%Y\%m\%d)
   ```

5. **监控异常登录**
   - 检查 `login_logs` 表
   - 设置失败的登录尝试警报

---

## 联系和支持

如有问题，请检查：
1. 日志文件
2. 数据库状态
3. 环境变量配置
4. 网络连接

---

**最后更新**: 2026-01-30
