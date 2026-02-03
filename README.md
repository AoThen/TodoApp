# TodoApp - 跨平台待办事项管理系统

TodoApp 是一个功能强大的跨平台任务管理应用，支持离线优先架构和实时同步。包含 Go 后端、React/TypeScript Web 前端和 Kotlin Android 原生应用。

## 核心特性

### 📋 任务管理
- 完整的任务 CRUD 操作
- 任务状态：待办 (todo)、进行中 (in_progress)、已完成 (done)、已归档 (archived)
- 优先级设置：低 (low)、中 (medium)、高 (high)
- 任务描述和截止时间支持

### 🔄 离线同步
- Delta 同步机制，实现无冲突离线/在线切换
- IndexedDB (Web) / Room (Android) 本地存储
- 自动同步后台任务
- 冲突检测与解决

### 🔐 安全认证
- JWT 访问令牌 + 刷新令牌机制
- 登录速率限制（15 分钟最多 5 次）
- 账户锁定机制（5 次失败锁定 30 分钟）
- bcrypt 密码哈希
- 完整的登录审计日志

### 🔔 实时通知
- WebSocket 实时推送
- 通知优先级：紧急、高、普通、低
- 已读/未读状态管理
- 批量标记已读
- 通知清理功能

### 📱 设备配对
- QR 码扫描配对新设备
- AES-256-GCM 端到端加密
- 多设备数据同步
- 配对密钥管理

### 👨‍💼 管理员面板
- 用户管理（创建、编辑、删除、锁定/解锁）
- 系统日志查看（登录日志、操作日志）
- 系统配置管理
- 数据导出（JSON/CSV）

## 平台功能

### Web 前端 (React/TypeScript)
- 响应式任务列表界面
- 在线状态实时显示
- 手动同步按钮
- 通知中心与未读计数
- 管理员面板（仅管理员可见）
- 设备配对 QR 码生成

### Android 原生应用 (Kotlin)
- Room 数据库持久化存储
- WorkManager 后台同步
- Jetpack UI 现代化界面
- 本地系统通知
- QR 码扫描器
- 登录/任务/配对/设置界面

## 快速开始

### 环境要求
- Go 1.19+
- Node.js 16+
- Android Studio (Android 开发)
- GCC (SQLite 需要)

### 1. 克隆项目
```bash
git clone https://github.com/yourusername/todoapp.git
cd todoapp
```

### 2. 设置环境变量
```bash
cp .env.example .env
# 编辑 .env 文件，设置 JWT_SECRET
export JWT_SECRET="$(openssl rand -hex 32)"
```

### 3. 启动后端
```bash
CGO_ENABLED=1 go run main.go
# 后端将在 http://localhost:8080 启动
```

### 4. 启动 Web 前端
```bash
cd web
npm install
npm start
# 前端将在 http://localhost:3000 启动
```

### 5. 构建 Android 应用
```bash
cd android
./gradlew assembleDebug
# APK 将生成在 android/app/build/outputs/apk/debug/
```

更多详细部署信息请参考 [DEPLOYMENT.md](DEPLOYMENT.md)。

## API 端点

所有端点位于 `/api/v1/` 前缀下。

### 认证
| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| POST | `/api/v1/auth/login` | 用户登录 | 否 |
| POST | `/api/v1/auth/refresh` | 刷新令牌 | Cookie |
| POST | `/api/v1/auth/logout` | 用户登出 | Cookie |

### 任务
| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| GET | `/api/v1/tasks` | 获取任务列表（支持分页） | 是 |
| POST | `/api/v1/tasks` | 创建新任务 | 是 |
| GET | `/api/v1/tasks/{id}` | 获取单个任务 | 是 |
| PATCH | `/api/v1/tasks/{id}` | 更新任务 | 是 |
| DELETE | `/api/v1/tasks/{id}` | 删除任务 | 是 |

### 同步
| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| POST | `/api/v1/sync` | Delta 同步 | 是 |
| WS | `/ws` | WebSocket 连接 | 是 |

### 通知
| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| GET | `/api/v1/notifications` | 获取通知列表 | 是 |
| POST | `/api/v1/notifications` | 创建通知 | 是 |
| PATCH | `/api/v1/notifications/{id}/read` | 标记为已读 | 是 |
| PATCH | `/api/v1/notifications/read-all` | 全部标记已读 | 是 |
| DELETE | `/api/v1/notifications/{id}` | 删除通知 | 是 |
| DELETE | `/api/v1/notifications/clear` | 清理旧通知 | 是 |
| GET | `/api/v1/notifications/unread-count` | 获取未读数量 | 是 |

### 管理员（需要管理员权限）
| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| GET | `/api/v1/admin/users` | 获取用户列表 | 管理员 |
| POST | `/api/v1/admin/users` | 创建用户 | 管理员 |
| PATCH | `/api/v1/admin/users/{id}` | 更新用户信息 | 管理员 |
| DELETE | `/api/v1/admin/users/{id}` | 删除用户 | 管理员 |
| POST | `/api/v1/admin/users/{id}/password` | 重置用户密码 | 管理员 |
| POST | `/api/v1/admin/users/{id}/lock` | 锁定用户 | 管理员 |
| POST | `/api/v1/admin/users/{id}/unlock` | 解锁用户 | 管理员 |
| GET | `/api/v1/admin/logs/login` | 获取登录日志 | 管理员 |
| GET | `/api/v1/admin/logs/actions` | 获取操作日志 | 管理员 |
| GET | `/api/v1/admin/config` | 获取系统配置 | 管理员 |
| PUT | `/api/v1/admin/config` | 更新系统配置 | 管理员 |

### 其他
| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| GET | `/api/v1/export` | 导出数据 (JSON/CSV) | 是 |
| GET | `/api/v1/health` | 健康检查 | 否 |

## 项目结构

```
todoapp/
├── main.go                          # Go 后端入口
├── internal/
│   ├── auth/                        # JWT 认证
│   ├── db/                          # 数据库操作
│   ├── response/                    # 统一响应格式
│   ├── validator/                   # 输入验证
│   ├── crypto/                      # 加密模块
│   └── websocket/                   # WebSocket 服务
├── web/
│   └── src/
│       ├── App.tsx                  # 主组件
│       ├── components/
│       │   ├── admin/               # 管理员面板
│       │   ├── notifications/       # 通知组件
│       │   └── DevicePairing.tsx    # 设备配对
│       ├── services/
│       │   ├── api.ts               # API 客户端
│       │   ├── indexedDB.ts         # 离线存储
│       │   ├── syncManager.ts       # 同步管理
│       │   ├── notification.ts      # 通知服务
│       │   ├── websocket.ts         # WebSocket 客户端
│       │   └── admin.ts             # 管理员 API
│       └── utils/
│           ├── debounce.ts          # 防抖工具
│           └── crypto.ts            # 加密工具
├── android/
│   └── src/main/java/com/todoapp/
│       ├── data/
│       │   ├── local/               # Room 数据库
│       │   ├── remote/              # Retrofit 网络层
│       │   ├── sync/                # WorkManager 同步
│       │   ├── crypto/              # 加密实现
│       │   └── notify/              # 通知管理
│       └── ui/                      # UI 界面
├── .env.example                     # 环境变量模板
├── AGENTS.md                        # 开发者指南
├── DEPLOYMENT.md                    # 部署指南
└── README.md                        # 项目说明
```

## 技术栈

### 后端
- Go 1.19+
- SQLite3 (WAL 模式)
- Gorilla Mux (路由)
- golang-jwt (JWT 认证)
- bcrypt (密码哈希)
- Gorilla WebSocket

### 前端
- React 18
- TypeScript
- Axios
- IndexedDB (idb)
- qrcode.react (QR 码生成)
- react-toastify (Toast 通知)

### Android
- Kotlin
- Jetpack Compose / ViewBinding
- Room (本地数据库)
- WorkManager (后台任务)
- Retrofit (网络请求)
- OkHttp (HTTP 客户端)
- ZXing (QR 扫描)

## 安全特性

- ✅ JWT 访问令牌 + HttpOnly 刷新令牌 Cookie
- ✅ 速率限制防止暴力破解
- ✅ 账户锁定机制
- ✅ 完整的登录审计日志
- ✅ bcrypt 密码哈希
- ✅ 输入验证和 SQL 注入防护
- ✅ CORS 保护
- ✅ WebSocket 加密连接
- ✅ AES-256-GCM 端到端加密（设备配对）

## 开发命令

### 后端 (Go)
```bash
go run main.go                              # 开发服务器
CGO_ENABLED=1 go build -o todoapp-server .  # 生产构建
go test ./...                               # 运行测试
go test -v ./internal/db -run TestInitDB    # 运行单个测试
```

### 前端 (React/TypeScript)
```bash
cd web
npm install && npm start                    # 开发服务器
npm run build                               # 生产构建
npm test                                    # 运行测试
npm test -- App.test.tsx                    # 运行单个测试
```

### Android (Kotlin)
```bash
cd android
./gradlew assembleDebug                     # Debug APK
./gradlew assembleRelease                   # Release APK
./gradlew test                              # 单元测试
./gradlew connectedAndroidTest              # 集成测试
```

### Docker
```bash
make build                                  # 构建镜像
make run                                    # 启动容器
make test                                   # 运行测试
make logs                                   # 查看日志
make clean                                  # 清理
```

## 数据库

TodoApp 使用 SQLite 作为存储引擎，具有以下表：

- `users` - 用户信息
- `tasks` - 任务数据
- `notifications` - 通知数据
- `delta_queue` - 离线更改队列
- `devices` - 已配对设备
- `login_logs` - 登录日志
- `tokens` - 刷新令牌存储
- `conflicts` - 冲突记录

## 故障排除

### 常见问题

**1. 编译错误 "cgo: C compiler not found"**
```bash
sudo apt-get install gcc  # Ubuntu/Debian
```

**2. JWT_SECRET 错误**
```bash
export JWT_SECRET="$(openssl rand -hex 32)"
```

**3. Android 模拟器连接问题**
- 使用 `10.0.2.2` 而非 `localhost` 访问主机服务

更多故障排除请参考 [DEPLOYMENT.md](DEPLOYMENT.md)。

## 贡献

欢迎提交问题和拉取请求！请确保：
- 代码符合项目风格指南（见 [AGENTS.md](AGENTS.md)）
- 运行所有测试
- 提交清晰的提交消息

## 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件

## 相关文档

- [AGENTS.md](AGENTS.md) - 开发者指南（给 AI 代理的编码指南）
- [DEPLOYMENT.md](DEPLOYMENT.md) - 详细部署指南
- [SECURITY_REVIEW.md](SECURITY_REVIEW.md) - 安全审查报告
- [ANDROID_INTEGRATION_GUIDE.md](ANDROID_INTEGRATION_GUIDE.md) - Android 集成指南
- [NOTIFICATION_IMPLEMENTATION_COMPLETE.md](NOTIFICATION_IMPLEMENTATION_COMPLETE.md) - 通知系统实现报告

---

**版本**: 2.0.0
**状态**: 生产就绪 ✅
