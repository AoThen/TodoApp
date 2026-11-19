# TodoApp UI 改版与工程化实施方案

> 基准：`prototype/web-prototype.html`、`prototype/android-prototype.html`（已确认的理想视觉稿，含深色模式）
> 状态：关键决策已确认（见 §8），待实施（批次①→④ 顺序推进）
> 验证约定：全程不做本地编译验证（Android 不构建），以静态代码审查 + 浏览器人工验收为准

## 1. 背景

- 项目完成度约 75%：三端主链路（CRUD → Delta 同步 → 冲突解决 → 通知 → 设备配对）真实可用，但存在交互缺口与工程化欠账。
- 原型沟通确认了目标 UI 形态：任务卡片点击打开编辑弹窗（标题/多行备注/状态/优先级/截止日期）、通知中心接入、深色模式、Web 分页与含已归档筛选、Android 底部导航 4 tab + 编辑底部弹层。
- 本方案将原型形态落地到 `web/`、`android/` 实际代码，并同步修复仓库分析发现的短板。

## 2. 原型确认的交互基准

| 决策点 | 确认结论 |
|---|---|
| 原型定位 | 理想视觉稿（现有功能基础上的精致 UI，可作改版参考） |
| 主题 | 浅色 + 深色模式切换（Web 顶栏按钮；Android 原型设置页含开关，本期以 values-night 跟随系统实现，手动开关后评估，见 §8-5） |
| 任务备注 | 多行文本，列表单行截断预览，完整内容在编辑弹窗查看修改 |
| 编辑入口 | **点击整张任务卡片**打开编辑（Web 居中弹窗 / Android 底部弹层）；勾选、删除按钮独立响应不触发编辑 |
| 编辑字段 | 标题、多行备注、状态（含已归档）、优先级、截止日期 |
| 新建与编辑 | 共用同一弹窗/弹层，按钮文字随模式切换 |
| 删除 | Web 删除 + 30 秒撤销 toast（撤销按钮仅删除场景显示）；Android 弹层内含删除入口 |
| Android 导航 | 底部 4 tab：任务 / 日历 / 通知 / 设置；FAB 新建 |
| 同步反馈 | 任何本地变更后触发「同步中… → 已同步」状态动画 |

## 3. 现状差距分析

| 能力 | 现状 | 差距 |
|---|---|---|
| Web 编辑弹窗 | 无任何编辑组件；`TaskForm.tsx` 仅新建+仅标题；`api.updateTask` 已有但闲置 | 需新建 `TaskEditModal`，含 5 字段 |
| Web 通知接入 | 组件树齐全（System/Bell/Center/Item + notification.ts），但 `App.tsx` 未引用（死代码） | 一处接线 |
| Web 备注展示 | `TaskItem.tsx` 不展示 description | 加单行截断预览 |
| Web 深色模式 | CSS 全部硬编码 hex，无 CSS 变量、无 theme 代码 | 从零引入变量体系 |
| Web 分页 | `api.getTasks` 与后端均支持 page/page_size，前端未传参无 UI | 加分页控件 |
| Web 测试凭据残留 | `App.tsx:422-423` defaultValue="test@example.com"/"password" | 移除 |
| 状态切换 | `handleToggleStatus` 已定义未绑定 UI | 绑定到状态徽章 |
| Android 导航 | 底部 3 tab（缺通知）；notificationFragment 已在 nav_graph | 加菜单项+图标 |
| Android 编辑 | 全屏 TaskDetailFragment 页内编辑；AddTaskFragment 仅新建；无 BottomSheet | 新建编辑 BottomSheet |
| Android 状态选择 | chips 均缺 archived（TaskDetail/AddTask 两处） | 补齐（4 态） |
| Android 深色 | DayNight parent 已有，缺 `values-night/` 色板 | 新建 night 色板 |
| Android 服务器地址 | `AppConfig.kt:11` 常量 `10.0.2.2:8080`；buildConfig 已开但无字段 | BuildConfig 注入 |
| 后端 PATCH | 支持 title/description/status/priority，**不支持 due_at** | 扩展 handler + SQL |
| 注册功能 | `handleRegister`（main.go:481-485）为 stub，路由已注释（设计为 admin 建号） | 文档对齐（本方案 §6 已处理） |
| 测试体系 | Web 零测试零配置；Android 零测试目录；Go 仅 13 个单测 | P2 全面覆盖 + GitHub Actions CI 执行（三端） |
| 仓库卫生 | test-build/todoapp 二进制与 web/build 为磁盘残留（已被 .gitignore 覆盖、未入库）；根目录 package.json 已入库且与项目无关；docker-compose JWT_SECRET 弱默认 | P2 清理 |

## 4. 分阶段改动方案

### P0：Web 核心交互落地

#### P0-1 任务编辑弹窗（新增）

**涉及文件**：
- 新建 `web/src/components/tasks/TaskEditModal.tsx` + `TaskEditModal.css`
- 修改 `web/src/App.tsx`、`web/src/components/tasks/TaskItem.tsx`、`web/src/components/tasks/TaskList.tsx`

**具体改动**：
1. `TaskEditModal.tsx`：props 为 `task: Task`、`onSave(updates: Partial<Task>)`、`onClose()`；字段：标题 input、备注 textarea（rows=4）、状态 select（todo/in_progress/done/archived 四态）、优先级 select、截止日期 `<input type="date">`；样式对齐原型（圆角弹窗、遮罩 backdrop-filter；颜色先沿用现有硬编码方式，P1-1 落地时随 CSS 变量统一改造）
2. `App.tsx`：新增 `editingTask` state；`TaskList` 透传 `onEditTask`；保存流程**不走** `api.updateTask` 直连，而是与现有删除/新增一致：更新 IndexedDB 本地任务 → `syncManager.enqueueLocalChange` 入 delta 队列 → 刷新列表 → 同步状态动画（保持 offline-first 架构不变）
3. `TaskItem.tsx`：卡片根节点 `onClick={() => onEditTask(task)}`；多选 checkbox 与删除按钮 `e.stopPropagation()`；description 增加单行截断展示（`text-overflow: ellipsis`）
4. 保存与关闭时弹窗字段复位，标题为空不允许保存

**验收标准**：点卡片弹窗且字段回显正确；保存后列表即时更新并出现同步状态变化；checkbox/删除不误触编辑；深浅主题下弹窗均可读。

#### P0-2 通知系统接入

**涉及文件**：`web/src/App.tsx`、`web/src/components/layout/Header.tsx`

**具体改动**：
1. `App.tsx` 在登录态布局中挂载 `NotificationSystem`（按组件实际 props 传入 token）
2. `Header.tsx` 增加 `NotificationBell`（未读红点），点击展开 `NotificationCenter`
3. `NotificationSystem.tsx` 内 `(window as any).toast` 全局 hack 替换为 `import { toast } from 'react-toastify'`（依赖已在 package.json，App.tsx 已在用）

**验收标准**：登录后铃铛可见、未读计数正确、可标记已读/全部已读；WebSocket 推送新通知时红点变化。

#### P0-3 杂项清理

1. `App.tsx:422-423`：移除登录表单 defaultValue，保留 placeholder
2. 状态徽章点击绑定 `handleToggleStatus`（todo ↔ done 快速切换，其余状态走编辑弹窗）
3. README/AGENTS.md 文档修正（见 §6）

### P1：体验与配套

#### P1-1 Web 深色模式

**涉及文件**：`web/src/App.css`、`web/src/components/notifications/` 下 3 个 css、`web/src/components/NotificationSystem.css`（位于 components/ 根目录）、`web/src/components/layout/Header.tsx`

**具体改动**：
1. `App.css` 顶部定义 `:root{--bg/--surface/--card/--border/--text/--muted/--primary/--success/--warning/--danger/--info/--shadow}`，`html[data-theme="dark"]` 全量覆盖（取值直接采用原型 web-prototype.html 的变量表）
2. 全局 CSS 中硬编码 hex 逐处替换为 `var()`（App.css 约 385 行 + 通知组件 4 个 css）
3. `Header.tsx` 加主题切换按钮：`document.documentElement.dataset.theme` 切换；localStorage 持久化；首次访问跟随 `prefers-color-scheme`

**验收标准**：切换即时生效、刷新后保持；登录页/任务/弹窗/通知抽屉/管理面板在深色下均可读。

#### P1-2 Web 分页

**涉及文件**：`web/src/App.tsx`、`web/src/services/api.ts`（如需透传）、新建 `web/src/components/tasks/Pagination.tsx`

**具体改动**：`loadTasks` 传入 `page/page_size`（与 TaskFilter 联动，筛选变化重置第 1 页）；列表下方渲染分页控件（上一页/页码/下一页 + 共 N 条），样式与任务卡片风格一致；后端无需改动（main.go:719-752 已支持）。

**验收标准**：翻页请求带分页参数；筛选切换回到第 1 页；空页不越界。

#### P1-3 Android 交互对齐

**涉及文件**：
- `android/src/main/res/menu/bottom_nav_menu.xml`（+drawable 图标）
- 新建 `android/src/main/java/com/todoapp/ui/tasks/TaskEditBottomSheet.kt`
- `TaskAdapter.kt`、`TaskListFragment.kt`、`AddTaskFragment.kt`、`TaskDetailFragment.kt`
- 新建 `android/src/main/res/values-night/colors.xml`
- `android/build.gradle.kts`、`android/src/main/java/com/todoapp/config/AppConfig.kt`

**具体改动**：
1. **4 tab**：`bottom_nav_menu.xml` 增加 notification 菜单项（nav_graph 已有 destination，仅需 menu + 图标）
2. **编辑底部弹层**：新建 `TaskEditBottomSheet : BottomSheetDialogFragment`，复用 AddTaskFragment 的字段逻辑（标题/描述/日期/状态/优先级），增加编辑模式（args 传 localId，ViewModel 回填）；`TaskAdapter` 卡片点击由跳详情页改为打开 BottomSheet；TaskDetailFragment 保留（作为长按或弹层内入口）
3. **状态 chips 补 archived**：TaskEditBottomSheet / AddTaskFragment / TaskDetailFragment 三处状态选择器统一为 4 态；同时排查 `Room` 实体 `isArchived` 冗余字段——落地方案统一以 `status="archived"` 为准，`isArchived` 作为派生兼容字段，P2 再评估迁移移除
4. **深色模式**：新建 `values-night/colors.xml`（Material3 深色色板，对应 `md_theme_light_*` 全套）；DayNight parent 已具备，零代码改动
5. **服务器地址注入**：`build.gradle.kts` defaultConfig 增加 `buildConfigField("String", "SERVER_URL", ...)`（debug 用 10.0.2.2:8080，release 用占位符由 CI 注入）；`AppConfig.DEFAULT_BASE_URL` 引用 `BuildConfig.SERVER_URL`，SharedPreferences 运行时覆盖逻辑保留

**验收标准**（静态审查 + 真机人工）：4 tab 切换正常；点卡片弹层编辑、保存后列表刷新且 WorkManager 同步入队；chips 4 态可选；系统深色模式下界面可读；release 构建不含硬编码地址。

#### P1-4 后端 PATCH 支持 due_at

**涉及文件**：`main.go`（handleUpdateTask，约 L842-870）、`internal/db/db.go`（UpdateTaskFull）

**具体改动**：
1. handleUpdateTask 解析增加 `DueAt *string`（指针区分「未传」与「置空」），校验日期格式
2. `UpdateTaskFull` SQL SET 子句增加 `due_at`
3. `internal/db/db_test.go` 补一个用例：PATCH 带/不带 due_at 的更新行为

**验收标准**：`JWT_SECRET=... go test ./internal/db` 通过；前端编辑弹窗保存截止日期后经 sync 落库正确。

**附带检查**：`internal/types/conflict.go` `PrioritizeStatus`（L57-81）不含 archived——排序场景确认归档任务不参与优先级计算是否为预期行为，若是则在代码注释与本文档标注。

### P2：工程化收尾

#### P2-1 测试体系（全面覆盖）+ CI 接入

**Web（vitest + @testing-library/react + jsdom + fake-indexeddb）**：
1. `web/package.json` 增加依赖与 `test` script（vitest run）
2. services：`syncManager`（enqueueLocalChange 入队/delta 应用/冲突回调）、`api`（mock axios：错误处理、分页参数透传）、`indexedDB`（任务 CRUD 与 delta 队列存取）、`notification`（未读数/已读）
3. 组件：`TaskEditModal`（回显/空标题拦截/保存回调）、`TaskItem`（渲染、点卡片编辑、checkbox/删除 stopPropagation）、`TaskFilter`、`Header` 铃铛未读红点、`NotificationCenter`（已读/全部已读）
4. App 集成冒烟：mock apiService，登录 → 列表渲染 → 翻页 → 编辑保存走 delta 队列

**Go（testing + httptest）**：
1. db 层：`UpdateTaskFull`（due_at 带/不传/置空三态）、`GetTasksFiltered`（分页边界、status/priority/search 组合）、delta 队列、冲突解决（含 PrioritizeStatus 对 archived 的语义确认）、批量操作
2. handlers：PATCH due_at、通知列表/未读数/已读（httptest + JWT 中间件）
3. auth/validator/utils：登录限流与锁定边界、CSV 流式导出；现有 13 个单测保持通过

**Android（JUnit + Robolectric，仅 CI 执行，本地不编译）**：
1. 建立 `android/src/test/`：`DeltaSyncWorkerTest`（delta 应用与上报）、`TaskRepositoryTest`、Room DAO（in-memory）、`AesGcmManagerTest`
2. ViewModel 状态测试（列表/编辑回填）

**CI（新建 `.github/workflows/ci.yml`）**：
1. `go-test` job：setup-go + `JWT_SECRET=test-secret-key-for-unit-tests-1234567890 go test -v -count=1 ./...`
2. `web-test` job：setup-node + `cd web && npm ci && npm test`
3. `android-test` job：setup-java + `cd android && ./gradlew test`（JVM 单测无需模拟器，配 gradle 缓存）
4. 触发：push + pull_request（main 分支）；AGENTS.md / README 测试命令随落地恢复

#### P2-2 main.go 拆分

在 P2-1 测试就位后执行，以现有行为测试保护纯移动重构。2681 行按域拆分为 `internal/handlers/`（auth/tasks/sync/notifications/devices/admin/export）+ middleware 文件；纯移动代码，不改行为；拆分后 `go build` 与现有测试作静态回归（不本地执行，交由 CI）。

#### P2-3 仓库卫生

1. 清理磁盘残留：直接删除 `test-build`、`todoapp`（各约 9.4MB 编译产物）与 `web/build/`——三者已被 `.gitignore` 覆盖、未被 git 跟踪（注意：勿用 `git rm`，会报 pathspec 错误）
2. `git rm package.json package-lock.json`：根目录残留的 npm 文件（仅含一个 ajv devDependency，与项目无关）确在版本库中，需移除
3. `docker-compose.yml` JWT_SECRET 移除弱默认值 `change-this-to-a-secure-secret-key`，改为必填注入（缺失即启动失败）

## 5. 实施顺序与依赖

```
P0-1 编辑弹窗 ──► P1-4 后端 due_at（弹窗含截止日期字段，需后端先支持或同批合入）
P0-2 通知接入（独立）
P0-3 清理（独立）
P1-1 深色模式（依赖 P0-1 的弹窗组件，同步改造其 CSS）
P1-2 分页（独立）
P1-3 Android 全套（独立于 Web，可与 P1-1/P1-2 并行）
P2 工程化（最后）
```

建议合入批次：① P0 全部 + P1-4；② P1-1 + P1-2；③ P1-3；④ P2-1（测试+CI）→ P2-2（main.go 拆分）→ P2-3（仓库卫生）。

## 6. 文档更新清单（已随本方案执行）

| 文档 | 更新内容 |
|---|---|
| `docs/ui_revamp_plan.md` | 本方案（新建） |
| `README.md` | 删除不存在的 `POST /api/v1/users/register` 端点行，注明账户由管理员创建；修正 Android APK 输出路径；web 测试命令标注测试体系待建；目录树新增 prototype/；相关文档增加本方案链接 |
| `AGENTS.md` | File Structure 修正（移除不存在的 internal/config/，补全 web/android 子目录，新增 prototype/、docs/）；前端 `npm test` 命令标注测试体系待建；Android 测试示例移除不存在的 DeltaSyncWorkerTest 具名用例 |

## 7. 风险与注意事项

1. **编辑保存走 delta 队列而非直连 PATCH**：必须与现有 offline-first 流程一致，否则会产生双写与同步冲突；`api.updateTask` 保持闲置或删除
2. **Android isArchived 双重表达**：status 字符串与布尔字段并存，归档写入时两处都要同步，否则 sync 上报字段不一致
3. **深色模式 CSS 改造面大**：App.css + 4 个通知 css 逐处替换，需逐屏人工验收（登录/任务/弹窗/抽屉/管理面板/配对）
4. **Web 筛选「已归档」与后端匹配语义**：`GetTasksFiltered` status 为精确匹配；status 未传时不过滤，「全部」视图**包含**归档任务——与原型「全部 tab 显示所有状态」的语义一致，无需后端改动
5. **原型与实现的样式差异**：原型为独立视觉稿，落地时沿用现有组件结构与 CSS 组织方式，仅对齐交互与主题变量，不做像素级复刻
6. **CI 首次跑通成本**：Android gradle 依赖下载与内存参数可能需调 `gradle.properties`/缓存；web 测试注意 fake-indexeddb 与现有 indexedDB 封装的兼容性
7. **JWT_SECRET 去弱默认改必填**：已部署的 docker 环境若未注入该变量将启动失败——合入前确认部署侧已配置

## 8. 实施决策记录（2026-11-19 与用户确认）

| # | 决策点 | 结论 |
|---|---|---|
| 1 | 编辑保存链路 | delta 队列（syncManager 已支持 update，不走 api.updateTask 直连） |
| 2 | due_at 落地 | 同批实现：编辑弹窗 + PATCH 扩展 + delta payload |
| 3 | Android 编辑形态 | 新建 TaskEditBottomSheet，详情页保留 |
| 4 | Web 通知 | 挂载现有 NotificationSystem，window.toast 替换为 react-toastify 标准引用 |
| 5 | 深色模式 | CSS 变量 + data-theme + localStorage 记忆，默认随系统；Android values-night 自动跟随（暂不加手动开关，原型设置页开关项后续评估） |
| 6 | 分页 | 页码分页，每页 20，筛选/搜索变化重置第 1 页 |
| 7 | 「全部」视图 | 保持含归档（与后端现状、原型语义一致，零改动） |
| 8 | main.go 拆分 | 纳入本次 P2-2 |
| 9 | 测试深度 | 全面覆盖（口径见 P2-1） |
| 10 | 仓库卫生 | 全做（含 JWT_SECRET 必填化） |
| 11 | CI | GitHub Actions 三端都跑（go-test / web-test / android-test） |
