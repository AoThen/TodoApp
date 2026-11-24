# Android 测试基础设施

本文说明 TodoApp Android 模块的三层测试体系：本地 JVM 单元测试、CI 上的单元测试任务、以及 GitHub Actions 模拟器上的 instrumented 测试。

## 测试分层

| 层级 | 位置 | 运行环境 | 触发方式 |
| --- | --- | --- | --- |
| JVM 单元测试 | `android/src/test/` | 本地 JVM / CI ubuntu runner | 每次 push / PR（ci.yml `android-test`） |
| Instrumented 测试 | `android/src/androidTest/` | GitHub Actions Android 模拟器（KVM） | push 到 main / 手动 workflow_dispatch（android-e2e.yml） |
| 真机联调 | `android/src/androidTest/` | 本地连接的真机/模拟器 | 手动执行 |

## 本地 JVM 单元测试

```bash
cd android
./gradlew test              # 全部单元测试
./gradlew testDebugUnitTest --no-daemon   # CI 使用的方式
./gradlew test --tests "*.AddTaskViewModelTest"   # 指定测试类
```

覆盖范围（`android/src/test/java/com/todoapp/`）：

- `utils/ResultTest` — Result 封装的状态判断、map/flatMap、runCatching、getOrThrow
- `utils/DateTimeUtilsTest` — ISO8601 解析、展示格式、相对时间、过期判断
- `data/crypto/KeyStorageTest` — Keystore 不可用时的明文兜底路径、存取/清除/配对判断
- `data/crypto/CryptoInterceptorTest` — 请求加密、路径排除规则（auth/health/admin/tasks GET/DELETE）、响应解密
- `data/crypto/AesGcmManagerTest` — AES-GCM 加解密往返、随机 IV、非法输入
- `data/notify/NotificationWebSocketTest` — 消息处理器注册/注销、断连安全
- `ui/auth/LoginViewModelTest`、`ui/tasks/AddTaskViewModelTest` — 表单校验与异步状态流转
- `data/repository/TaskRepositoryTest`、`data/sync/*` — 仓库与增量同步

技术栈：JUnit4 + Robolectric（`@Config(sdk = [34])`）+ MockK + kotlinx-coroutines-test。协程测试统一使用 `Dispatchers.setMain(UnconfinedTestDispatcher())` 模式。

## CI 单元测试任务

`.github/workflows/ci.yml` 的 `android-test` job 在每次 push/PR 时执行：

1. `./gradlew testDebugUnitTest --no-daemon`
2. `./gradlew lint --no-daemon`

## 模拟器 instrumented 测试（android-e2e.yml）

GitHub 托管 runner 自带 `/dev/kvm`，只需通过 udev 规则放行权限：

```
KERNEL=="kvm", GROUP="kvm", MODE="0666", OPTIONS+="static_node=kvm"
sudo udevadm control --reload
sudo udevadm trigger --name-match=kvm
```

流程（`.github/workflows/android-e2e.yml`）：

1. 启动 Go 后端于宿主机 8080 端口（SQLite 落盘在 runner 工作目录）。
2. `reactivecircus/android-emulator-runner@v2.38.0` 启动 api-level 34 / x86_64 / google_apis 模拟器。
3. 在模拟器内执行 `./gradlew connectedDebugAndroidTest -PSERVER_URL=http://10.0.2.2:8080/api/v1/`。
   模拟器通过 `10.0.2.2` 回环访问宿主机后端。
4. 测试报告以 artifact 上传（`android/build/reports/androidTests/` 与 `android/build/outputs/androidTest-results/`）。

现有 instrumented 测试：`TaskDaoInstrumentedTest`、`NotificationDaoInstrumentedTest`（Room DAO 层）。

## 真机 / 本地模拟器手动执行

```bash
cd android
./gradlew connectedDebugAndroidTest \
  -PSERVER_URL=http://<电脑局域网IP>:8080/api/v1/
```

真机需与电脑同一网段，`SERVER_URL` 不能使用 `10.0.2.2`（该地址仅模拟器宿主回环有效）。

> 注：真机上完整的人工验收流程（环境搭建、登录配对、P0/P1/P2 用例矩阵、代码链路审查表）见 [android-device-acceptance.md](android-device-acceptance.md)。该文档推荐 `adb reverse` + `localhost` 方案解决真机明文访问问题。

## 常见失败排查

- **模拟器启动超时**：确认 KVM 步骤生效（`kvm-ok` 输出 `/dev/kvm is accessible`）。
- **instrumented 测试全部连接失败**：检查后端健康检查步骤日志，以及 `-PSERVER_URL` 是否指向正确地址。
- **本地跑单元测试报 Keystore 相关异常**：JVM 环境没有 AndroidKeyStore，`KeyStorage.init()` 会按设计降级，明文兜底路径即测试目标之一，不是环境问题。
