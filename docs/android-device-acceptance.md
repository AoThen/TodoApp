# Android 真机验收人工测试流程

> 适用范围：TodoApp Android 原生应用在**真机**上的人工验收。
> 与自动化测试的分工：JVM 单元测试（`android/src/test/`）、CI 模拟器 instrumented 测试（`android/src/androidTest/`）已在 [android-test-infra.md](android-test-infra.md) 说明；本文补充**真机上必须人工核对**的 UI 交互、配对加密、实时通知、离线同步等链路。

## 1. 定位与边界

- 覆盖：冷启动守卫、登录、设备配对、任务增删改查、搜索、日历、通知、设置、手动/周期同步、离线与跨端同步、异常与权限。
- 不覆盖：服务端接口正确性（Go 集成测试已覆盖）、界面视觉像素级对比（以功能可用与状态反馈正确为准）。
- 验证方式：按第 4 节用例逐条操作，结合第 5 节「代码链路审查表」静态核对；发现的问题按第 6 节记录。

## 2. 环境与构建

### 2.1 前置条件

- 真机：Android 8.0（API 26）及以上，arm64，USB 调试已开启（或 Android 11+ 无线调试）。
- 电脑：Go 1.20+、Node.js 18+、JDK 17、Android SDK（adb 可用）。
- 账号：后端首次启动需创建管理员（`INITIAL_ADMIN_EMAIL/PASSWORD`）。

### 2.2 后端启动

```bash
JWT_SECRET=<至少32字符> \
ENCRYPTION_KEY=<64位hex，与网页端一致> \
INITIAL_ADMIN_EMAIL=admin@example.com \
INITIAL_ADMIN_PASSWORD=<强密码> \
ENVIRONMENT=development \
go run .   # 默认监听 8080
```

### 2.3 网页前端启动（配对用）

```bash
cd web
npm install && npm start   # 默认 http://localhost:3000，后端地址取自 VITE_API_URL
```

### 2.4 构建与安装 App（关键）

**首次安装必须构建时注入正确的服务端地址**：App 的默认地址指向模拟器专用地址 `10.0.2.2`，真机不可达；而登录接口在配对完成前就会请求该地址，因此不注入会导致「未配对 → 登录失败 → 无法配对」的死锁。

推荐方案（**`adb reverse` + localhost**）：

```bash
cd android
adb reverse tcp:8080 tcp:8080        # 真机 localhost:8080 转发到电脑 8080
./gradlew assembleDebug \
  -PSERVER_URL=http://localhost:8080/api/v1/
adb install -r build/outputs/apk/debug/android-debug.apk
```

> **为什么不用电脑局域网 IP**：`network_security_config.xml` 仅对 `10.0.2.2` 与 `localhost` 放行明文 HTTP，局域网 IP 会被明文策略拦截，连接直接失败。`adb reverse` 方案无需改任何配置，`localhost` 已在白名单内。
>
> **二维码 server 字段一致性**：网页端配对二维码里的 server 取自后端 `SERVER_URL` 环境变量（未设置时默认 `http://localhost:8080`）。保持后端不设置 `SERVER_URL`，二维码 server 即为 `localhost`，与 `adb reverse` 匹配；若后端显式设置局域网 IP，二维码将携带不可达地址，手机明文访问会被拦截。

其他说明：

- 配对成功后 App 会将配对页回填的地址持久化，后续登录/同步均走该地址，重启 App 亦恢复（`TodoApp.onCreate` 恢复）。
- 刷新接口地址的路径会自动补 `/api/v1/` 后缀，构建时传入是否带后缀均可。
- **release 包**：强制 HTTPS，必须传 `-PSERVER_URL=https://<host>/api/v1/`，自签证书需额外信任配置，不在本期验收范围。

### 2.5 常用命令

```bash
adb devices                                    # 确认设备
adb shell am start -n com.todoapp/.ui.MainActivity
adb logcat --pid=$(adb shell pidof -s com.todoapp)   # 抓应用日志
adb shell pm clear com.todoapp                 # 清应用数据（重测配对/登录）
adb uninstall com.todoapp
adb reverse --remove-all                       # 撤销端口转发
```

## 3. 登录与配对

1. **顺序**：先登录、后配对。配对接口需要登录态，未登录时配对页会提示并引导至登录页；登录成功后若尚未配对，自动回到配对页。
2. **扫码配对（主路径）**：网页登录同一账号 → 顶栏「设备配对」→ 展示二维码 → 手机配对页点「扫描二维码」→ 授权相机 → 对准二维码 → 校验通过后完成绑定并进入任务列表。
3. **手动配对（备用）**：点「手动配对」→ **粘贴二维码的 JSON 原文**（非短码）→ 完成绑定。适合无法扫码或二维码过期时使用；注意手动配对仅在本地保存通道密钥，服务端设备列表不会登记该设备。
4. **异常分支**：二维码过期（提示已过期，请刷新后重扫）；二维码非 TodoApp 配对码（提示无效二维码）；旧版二维码缺少通道加密密钥（提示二维码版本过旧，需网页端重新生成）；release 包扫到明文 http 地址（提示无效二维码，release 仅接受 https）。
5. **登录分支**：空邮箱/非法邮箱/空密码/短密码即时提示且登录按钮不可用；密码错误提示登录失败；登录成功后若已配对直接进入任务列表。

## 4. 验收用例矩阵

> 操作前先按 2.2–2.4 完成环境；每条记录「步骤 / 预期 / 实际 / 结论（通过/失败/阻塞）」。

### P0 主链路（17 条）

| 编号 | 用例 | 步骤要点 | 预期 |
| --- | --- | --- | --- |
| A01 | 冷启动守卫（未配对） | 清数据后首次启动 | 进入配对页，底部导航隐藏；不经任何入口能进入任务列表 |
| A02 | 登录表单校验 | 空邮箱、非法邮箱、空/短密码逐项输入 | 对应字段错误提示，登录按钮不可点；修正后可点 |
| A03 | 登录失败 | 输入错误密码登录 | 提示登录失败，按钮恢复可点，不跳主界面 |
| A04 | 登录成功（未配对） | 正确账号密码登录 | 提示成功并回到配对页；通知权限被申请（Android 13+） |
| A05 | 登录成功（已配对） | 已配对状态下登录 | 提示成功，直接进入任务列表 |
| A06 | 扫码配对 | 网页生成二维码，手机扫码 | 相机授权后完成配对，提示成功，进入任务列表；网页设备列表出现该 Android 设备 |
| A07 | 扫码异常-过期 | 等待二维码超过有效期后扫码 | 提示二维码已过期 |
| A08 | 扫码异常-无效码 | 扫非配对二维码 | 提示无效二维码 |
| A09 | 手动配对 | 粘贴二维码 JSON 原文 | 提示配对成功，进入任务列表 |
| A10 | 新建任务 | 点 FAB → 输入标题/描述/状态/优先级/截止日期 → 创建 | 提示创建成功，返回列表并显示新任务 |
| A11 | 新建校验与未保存 | 标题留空/超长、描述超长；输入后点返回 | 错误提示，按钮禁用；有输入时返回弹出「放弃更改」确认 |
| A12 | 列表与搜索 | 查看列表；搜索命中与无结果关键词 | 列表正常；命中过滤标题/描述；无结果时显示「没有匹配的任务」空态 |
| A13 | 勾选完成 | 勾选任务卡片完成；再取消勾选 | 状态变为已完成并入同步队列；取消后恢复原状态 |
| A14 | 编辑弹层 | 点卡片打开弹层 | 字段正确回显；修改后保存，列表刷新并提示更新成功 |
| A15 | 编辑-清除截止日期 | 清空截止日期保存 | 截止日期被清空（非保留旧值），同步后生效 |
| A16 | 删除任务 | 弹层内删除 → 确认 | 二次确认后任务移除，提示删除成功 |
| A17 | 手动同步 | 联网状态下点「立即同步」 | 按钮/进度显示同步中 → 同步成功提示，列表为最新 |

### P1 离线与同步专项（8 条）

| 编号 | 用例 | 步骤要点 | 预期 |
| --- | --- | --- | --- |
| B01 | 断网创建后联网同步 | 飞行模式下新建任务，恢复网络后手动同步 | 任务同步到服务端（网页端核对可见） |
| B02 | 离线编辑再同步 | 断网编辑已有任务再同步 | 离线修改内容完整同步，不被服务端旧值覆盖 |
| B03 | 离线删除再同步 | 断网删除任务再同步 | 服务端同步删除，本地列表移除 |
| B04 | 跨端双向 | 网页建任务→手机同步；手机建任务→网页可见 | 双向一致 |
| B05 | 冲突数据核对 | 两端同时改同一任务后分别同步 | 服务端按合并规则处理，最终值可解释，不出现数据丢失（冲突仅落库，无界面提示） |
| B06 | token 过期续期 | 等待访问令牌过期（默认 15 分钟）后操作 | 无感续期，不中断使用，无登出提示 |
| B07 | 刷新令牌失效 | 使刷新令牌失效后操作 | 提示登录已过期并跳转登录页 |
| B08 | 周期同步 | 联网静置 15 分钟以上 | 周期任务自动同步，数据与服务端一致 |

### P2 异常与适配（8 条）

| 编号 | 用例 | 步骤要点 | 预期 |
| --- | --- | --- | --- |
| C01 | 相机权限拒绝 | 扫码时拒绝相机权限 | 出现权限说明弹窗，引导再次授权；不崩溃 |
| C02 | 通知权限拒绝 | Android 13+ 拒绝通知权限 | 界面操作正常，系统通知静默不弹出（列表仍可见） |
| C03 | 深色模式 | 设置中切换深色开关 | 全页面（登录/配对/任务/弹层/日历/通知/设置）可读，重启后保持 |
| C04 | 4 tab 与返回栈 | 四个 tab 反复切换并返回 | 各 tab 状态正确，返回栈不出现跳回失效页 |
| C05 | 断网启动 | 飞行模式下启动 App | 本地已有数据正常展示，离线状态提示正确 |
| C06 | 导入非法数据 | 导入非合法 JSON 文件 | 提示导入失败，不写入脏数据 |
| C07 | 导入部分非法 | 含部分非法条目的合法 JSON | 合法条目导入成功，提示跳过数量 |
| C08 | 多账号隔离 | 退出登录换账号登录 | 前账号任务不可见，退出时待同步队列被清空，不串号 |

## 5. 代码链路审查表

> 静态核对每条用例对应的代码路径，结论 ✅ 通过 / ⚠️ 存在问题。审查时间：2026-11-24，仅静态审查（未本地编译）。

| 用例 | 链路 | 代码位置 | 结论 | 说明 |
| --- | --- | --- | --- | --- |
| A01 | 启动守卫/底栏隐藏 | `MainActivity.kt:66-93`、`NavigationGuard.kt:11-19`、`nav_graph.xml`（startDestination=配对页） | ✅ | 守卫覆盖导航分发、底栏点击与恢复；配对/登录页隐藏底栏 |
| A02-A05 | 登录校验/状态 | `LoginViewModel.kt:45-134`、`LoginFragment.kt:63-124` | ✅ | 邮箱格式、密码长度校验与按钮禁用联动；失败提示；成功后按配对状态分流 |
| A06 | 扫码解析与校验 | `QRScannerActivity.kt:124-227`、`PairingFragment.kt:143-160` | ✅ | 依次校验类型/版本/地址(release 仅 https)/密钥格式/通道密钥/过期；成功后回传配对页统一处理 |
| A06 | 扫码配对服务端登记 | `QRScannerActivity.kt:175-204` + 后端 `/devices/pair` | ✅ | 扫码页调用服务端登记设备；**网页端已修复**：二维码钥匙不再被网页自身消费（`DevicePairing.tsx:54-97`），不再出现 409「该设备已配对」 |
| A07-A08 | 扫码异常分支 | `QRScannerActivity.kt:299-369` | ✅ | 过期/无效码/旧版缺通道密钥各有对应提示与重试 |
| A09 | 手动配对 | `PairingFragment.kt:173-190`、`PairingViewModel.kt:45-109` | ⚠️ | 粘贴二维码 JSON 后仅在本地保存通道密钥，**不调服务端登记**，网页设备列表无记录（已知限制）；另手动配对框提示文案为「请输入配对代码」，实际需粘贴 JSON 原文，文案易误导 |
| A05/A09 | 地址覆盖持久化 | `AppConfig.kt:44-57`、`RetrofitClient.kt:242-259`、`TodoApp.kt:20-21` | ✅ | 配对后即时切换并持久化，重启恢复；切换时重置网络实例、清会话与 Cookie |
| A10-A11 | 新建任务校验/未保存 | `AddTaskViewModel.kt:47-115`、`AddTaskFragment.kt:89-140、260-269` | ✅ | 标题必填/≤200、描述≤2000；有输入时返回弹出放弃确认；防快速连击 |
| A10 | 截止日期入库 | `AddTaskViewModel.kt:100-105` | ✅ | 日期补 `T00:00:00Z`，日历按该日期归组 |
| A12 | 列表/搜索/空态 | `AppDatabase.kt:90`、`TaskRepository.kt:25`、`TaskListViewModel.kt:192-198`、`TaskListFragment.kt:196-209` | ✅ | 按用户过滤、排除已删除；搜索命中标题/描述；区分空任务与无匹配 |
| A13 | 勾选完成 | `TaskRepository.kt:91-116`、`TaskListViewModel.kt:113-122` | ✅ | 完成记录 `previousStatus`，取消勾选可恢复进行中/已归档等原状态；入同步队列 |
| A14-A15 | 编辑弹层回显/保存 | `TaskDetailViewModel.kt:52-171`、`TaskEditBottomSheet.kt:105-145` | ✅ | 回显五字段；清除日期置 null 而非保留旧值 |
| A14 | 编辑保存校验失败后再保存 | `TaskDetailViewModel.kt:134-145`、`TaskEditBottomSheet.kt:107-116` | ⚠️ | **确认缺陷**：保存遇校验失败后 `uiState` 停在 Error，修正字段后点保存被 `getCurrentTask()` 空判静默拦截，按钮无反馈（见第 7 节 I-01） |
| A16 | 删除任务 | `TaskDetailViewModel.kt:173-190`、`TaskRepository.kt:78-89`、`TaskEditBottomSheet.kt:69-80` | ✅ | 二次确认；软删除+入同步队列；删除成功提示与保存提示区分 |
| A17/B01-B04 | 手动/周期同步 | `TaskListViewModel.kt:124-178`、`DeltaSyncWorker.kt:29-279`、`AppConfig.kt:71-80` | ✅（见 B08 备注） | 唯一名+KEEP 防重复入队；insert 未确认时延迟 update/delete；滞留 delta 清理；服务端变更按 serverId/localId 合并；冲突落库 |
| B05 | 冲突处理 | `DeltaSyncWorker.kt:247-257` | ✅ | 冲突写入本地冲突表，无界面展示（已知限制） |
| B06/B07 | 令牌续期/失效 | `RetrofitClient.kt:102-146`、`AuthInterceptor.kt`、`MainActivity.kt:113-146` | ✅ | 401 在锁内单飞刷新并重试一次；失效清会话跳登录；前台过期静默续期 |
| B08 | 周期同步与手动同步观察 | `DeltaSyncWorker.schedule`、`TaskListViewModel.observeSyncWork` | ⚠️ | **提示**：无网络时手动同步任务被约束挂起，观察流停留在「同步中」且无失败反馈；周期任务由 WorkManager 重试兜底，长时间离线后首次联网可恢复（见第 7 节 I-03） |
| A04/通知 | 通知实时通道 | `MainActivity.kt:36-45`、`NotificationSocketController.kt:26-60`、`NotificationWebSocket.kt:51-150` | ✅ | 前台/登录/配对成功后建连；握手协商加密；加密字节消息用通道密钥解密后按类型分发 |
| 通知 | 通知落库与系统通知 | `NotificationManager.kt:55-106` | ✅ | 收到推送写本地库并弹系统通知；Android 13+ 无权限时静默跳过系统通知 |
| 通知 | 通知列表与操作 | `NotificationFragment.kt:52-190` | ⚠️ | 列表仅本地库且固定取第 1 页（20 条），无翻页入口（见第 7 节 I-02）；标记已读/全部已读/删除/清空逻辑正确 |
| C01 | 相机权限 | `QRScannerActivity.kt:68-80`、`PairingFragment.kt:126-141` | ✅ | 权限缺失申请；被拒显示说明弹窗引导 |
| C02 | 通知权限 | `LoginFragment.kt:115-124`、`NotificationFragment.kt:76-82` | ✅ | 登录成功即申请，通知页再次确认；拒绝不阻断界面 |
| C03 | 深色模式 | `SettingsFragment.kt:82-94`、`ThemeUtils` | ✅ | 开关持久化，应用启动恢复 |
| C04 | 4 tab/返回栈 | `bottom_nav_menu.xml`、`MainActivity.kt:148-155` | ✅ | 菜单与导航图目的地一致；导航守卫统一兜底 |
| C05 | 断网启动 | `TaskListViewModel` 网络回调、`TaskListFragment.kt:211-241` | ✅ | 网络状态实时反馈，离线 chip/状态文案正确 |
| C06/C07 | 导入导出 | `SettingsFragment.kt:150-222`、`ImportExportManager` | ✅ | 导出 JSON/CSV 含任务数量提示；导入逐条校验，部分非法时提示成功与跳过数 |
| C08 | 多账号隔离 | `TaskRepository.getCurrentUserId`、`SettingsFragment.kt:134-148` | ✅ | 任务查询按当前用户过滤；退出登录清待同步队列与通知通道，防跨账号串写 |

## 6. 判定标准与缺陷记录

- **分级**：
  - P0 阻塞发布：崩溃、数据丢失、登录/配对不可用、同步错乱、多账号串号。
  - P1 功能问题：核心功能不可用或结果错误，但有绕行方式。
  - P2 体验问题：提示文案、无翻页、无角标等。
- **记录模板**：

  | 字段 | 内容 |
  | --- | --- |
  | 用例编号 | A/B/C 编号 |
  | 现象描述 | 操作步骤 + 实际表现 |
  | 复现条件 | 数据/网络/构建类型 |
  | 截图/日志 | 关键界面截图 + `adb logcat` 片段 |
  | 结论 | 通过 / 失败 / 阻塞 |

## 7. 已知问题与限制（静态审查结论）

| 编号 | 位置 | 描述 | 建议 |
| --- | --- | --- | --- |
| I-01 | `TaskDetailViewModel.kt:134-145` | 编辑弹层保存遇校验失败后界面停留错误态，修正字段再点保存被静默拦截（`getCurrentTask()` 仅 Success 状态返回任务） | 修正后应将 `uiState` 复位为 Success，或保存前直接从表单字段构建任务 |
| I-02 | `NotificationFragment.kt:31-33` | 通知列表仅展示第 1 页（20 条），无加载更多 | 补翻页控件或调大单页 |
| I-03 | `TaskListViewModel.kt:154-178` | 无网络时手动同步任务挂起，「同步中」无超时/失败反馈 | 观察流补充超时或失败状态回写 |
| L-01 | `PairingFragment.kt:173-190` | 手动配对不在服务端登记设备，网页设备列表无记录 | 文档标注；如需登记可补调 `/devices/pair` |
| L-02 | `strings.xml:30`（`pairing_code_hint`） | 手动配对提示「请输入配对代码」，实际需粘贴二维码 JSON 原文 | 文案改为「请粘贴配对二维码内容」 |
| L-03 | 通知页/底栏 | 无未读角标；通知仅来自实时推送落库，不拉服务端历史通知 | 产品决策是否补齐 |
| L-04 | 日历 | 任务按截止日期归组，无截止日期任务不显示 | 与产品约定一致，文档标注 |

## 8. 附录：造数命令

**触发服务端推送通知（用于通知验收）**：在网页端删除任务会触发「任务已删除」实时推送，手机应收到系统通知并出现在通知列表。

```bash
# 登录取访问令牌
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"<密码>"}' | jq -r .access_token)

# 批量删除任务（触发 tasks_deleted 实时推送）
curl -s -X DELETE http://localhost:8080/api/v1/tasks/batch \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"task_ids":[1]}'
```

**生成未消费的配对二维码 JSON（需要扫码页能力时）**：

```bash
curl -s -X POST http://localhost:8080/api/v1/devices/pair/init \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"device_type":"android"}'
# 取返回的 key/encryption_key/server_url/expires_at 组装 JSON，
# 用任意二维码生成工具转成图片后扫码
```
