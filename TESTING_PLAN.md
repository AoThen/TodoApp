# TodoApp 测试覆盖率提升计划

## 现状分析
后端（Go）已具备认证、任务、同步、通知、设备配对、管理员等集成测试与单元测试；
前端（Vitest + React Testing Library）覆盖全部组件目录与服务层，另有 Playwright 冒烟 E2E；
Android 具备 6 个单元测试与 2 个 instrumentation 测试（后者受 CI 无 KVM 限制无法云端执行）。
当前重点是补齐跨平台同步 E2E 与管理员 E2E 场景。

## 测试策略

### 1. 单元测试（Unit Tests）
**目标**: 为所有核心函数和方法提供100%的单元测试覆盖

#### 后端单元测试（Go）
- **验证器函数**: `internal/validator/validator.go`
  - 邮箱验证
  - 密码验证
  - 任务标题验证

- **数据库操作**: `internal/db/*.go`
  - 任务CRUD操作
  - 用户管理操作
  - 通知管理操作
  - 同步相关操作

- **认证模块**: `internal/auth/jwt.go`
  - 令牌生成
  - 令牌验证
  - 用户认证

#### 前端单元测试（TypeScript/React）
- **API服务**: `web/src/services/api.ts`
  - 登录功能
  - 任务操作
  - 同步功能

- **同步管理器**: `web/src/services/syncManager.ts`
  - 本地变更排队
  - 同步逻辑

- **React组件**: `web/src/components/`
  - 任务列表组件
  - 表单组件
  - 管理面板组件

### 2. 集成测试（Integration Tests）
**目标**: 测试系统组件之间的交互

#### API端点测试
- 认证端点
- 任务管理端点
- 用户管理端点
- 同步端点
- 管理员端点

#### 数据库集成测试
- 完整的CRUD操作流程
- 事务处理
- 批量操作

### 3. 端到端测试（E2E Tests）
**目标**: 测试完整的用户工作流程

#### 优先级用例
1. 用户注册、登录、登出
2. 任务创建、编辑、删除
3. 任务同步（在线/离线模式）
4. 管理员功能

## 实施计划

### Phase 1: 基础测试框架搭建（第1周）
- [x] 配置Go测试环境
- [x] 配置前端测试环境（Vitest, React Testing Library）
- [x] 设置CI/CD测试管道

### Phase 2: 高优先级单元测试（第2-3周）
- [x] 认证/授权功能测试
- [x] 核心数据库操作测试
- [x] 关键验证器函数测试

### Phase 3: 集成测试（第4-5周）
- [x] API端点集成测试
- [x] 数据库集成测试
- [x] 同步功能测试

### Phase 4: 前端测试（第6-7周）
- [x] React组件测试
- [x] 前端服务测试
- [x] UI交互测试

### Phase 5: 端到端测试（第8周）
- [x] 用户场景测试（Playwright 冒烟：登录→建任务→状态流转→删除→登出）
- [x] 跨平台同步测试（Playwright：Web↔服务端↔B设备双向同步、冲突保留本地/使用服务器、删除同步、离线补同步）
- [ ] 管理员功能测试（E2E 层面）

## 测试覆盖率目标
- **单元测试**: 80%+ 行覆盖率
- **集成测试**: 覆盖所有API端点
- **E2E测试**: 覆盖所有主要用户工作流程

## 工具和技术栈
- **后端**: Go testing package
- **前端**: 
  - 单元测试: Jest + React Testing Library
  - E2E测试: Cypress 或 Playwright
- **覆盖率报告**: go tool cover, Istanbul/NYC for JS

## 预期收益
- 提高代码质量
- 减少回归错误
- 提高开发信心
- 支持持续集成/部署