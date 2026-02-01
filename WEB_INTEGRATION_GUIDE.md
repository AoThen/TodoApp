# Web前端集成指南

## 快速开始

### 步骤1：安装依赖

```bash
cd web
rm -rf node_modules package-lock.json
npm install
```

### 步骤2：集成到App.tsx

在`App.tsx`中添加以下代码：

```tsx
import React, { useState, useEffect } from 'react';
import NotificationSystem from './components/NotificationSystem';

function App() {
  const [authToken, setAuthToken] = useState<string | null>(
    localStorage.getItem('access_token')
  );

  // 登录成功后更新token
  const handleLogin = async (email: string, password: string) => {
    try {
      const { access_token } = await apiService.login(email, password);
      setAuthToken(access_token);
      localStorage.setItem('access_token', access_token);
    } catch (error) {
      console.error('Login failed:', error);
    }
  };

  // 渲染组件
  return (
    <div className="app">
      {/* 顶部导航栏 */}
      <header className="app-header">
        <h1>TodoApp</h1>

        {/* 通知系统 - 添加这里 */}
        {authToken && <NotificationSystem token={authToken} />}

        {/* 其他导航元素 */}
        <button onClick={handleLogout}>登出</button>
      </header>

      {/* 主要内容区域 */}
      <main className="app-main">
        {/* 你的应用内容 */}
      </main>
    </div>
  );
}

export default App;
```

### 步骤3：添加CSS（如果需要）

确保在`App.css`中包含通知overlay的样式（如果NotificationSystem.css没有自动引入）

```css
/* 在App.css中或在index.tsx中引入 */
@import './components/NotificationSystem.css';
```

## 完整示例

### 带通知系统的完整App.tsx示例

```tsx
import React, { useState, useEffect } from 'react';
import { apiService } from './services/api';
import NotificationSystem from './components/NotificationSystem';

const App: React.FC = () => {
  const [authToken, setAuthToken] = useState<string | null>(
    localStorage.getItem('access_token')
  );
  const [userEmail, setUserEmail] = useState<string>(
    localStorage.getItem('user_email') || ''
  );

  useEffect(() => {
    // 检查用户是否已登录
    if (authToken) {
      // 可以在这里获取用户信息
    }
  }, [authToken]);

  const handleLogin = async (email: string, password: string) => {
    try {
      const { access_token } = await apiService.login(email, password);
      setAuthToken(access_token);
      setUserEmail(email);
      localStorage.setItem('access_token', access_token);
      localStorage.setItem('user_email', email);
    } catch (error) {
      console.error('Login failed:', error);
      alert('登录失败');
    }
  };

  const handleLogout = async () => {
    try {
      await apiService.logout();
      setAuthToken(null);
      setUserEmail('');
      localStorage.removeItem('access_token');
      localStorage.removeItem('user_email');
    } catch (error) {
      console.error('Logout failed:', error);
    }
  };

  if (!authToken) {
    // 登录视图
    return (
      <div className="login-screen">
        <h1>TodoApp 登录</h1>
        <form onSubmit={(e) => {
          e.preventDefault();
          const formData = new FormData(e.currentTarget);
          handleLogin(
            formData.get('email') as string,
            formData.get('password') as string
          );
        }}>
          <input name="email" type="email" placeholder="邮箱" required />
          <input name="password" type="password" placeholder="密码" required />
          <button type="submit">登录</button>
        </form>
      </div>
    );
  }

  // 主应用
  return (
    <div className="app">
      {/* Header */}
      <header className="app-header">
        <div className="header-brand">
          <h1>TodoApp</h1>
        </div>

        <div className="header-actions">
          {/* 通知系统集成 */}
          <NotificationSystem token={authToken} />

          <span className="user-email">{userEmail}</span>
          <button onClick={handleLogout}>登出</button>
        </div>
      </header>

      {/* Main Content */}
      <main className="app-main">
        {/* 你的应用内容 */}
      </main>
    </div>
  );
};

export default App;
```

## 测试集成

### 1. 测试通知铃铛
```bash
# 启动应用
cd web
npm start

# 登录后，你应该能看到通知铃铛图标
```

### 2. 创建测试通知
```bash
# 使用curl创建测试通知
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "test",
    "title": "测试通知",
    "content": "这是一条测试通知",
    "priority": "normal"
  }'

# 你应该能在Web应用中看到新通知，并且未读数量增加
```

### 3. 测试实时推送
```bash
# 在另一个终端或浏览器开发者工具的Network标签中
# 观察WebSocket连接是否建立

# 发送通知后，应该能实时看到通知弹出
```

## 故障排除

### 问题1：通知铃铛不显示
**解决方案**：
- 检查是否正确导入了`NotificationSystem`组件
- 检查是否传递了正确的token
- 检查浏览器控制台是否有错误

### 问题2：WebSocket连接失败
**解决方案**：
- 确保后端服务器正在运行
- 检查是否配置了正确的JWT token
- 检查浏览器控制台的错误信息
- 确认加密密钥配置正确

### 问题3：通知不更新
**解决方案**：
- 检查浏览器控制台的WebSocket消息
- 检查`fetchUnreadCount`函数是否被调用
- 检查API响应格式是否正确

### 问题4：样式错误
**解决方案**：
- 确保CSS文件已正确引入
- 检查是否有CSS冲突
- 清除浏览器缓存

## 样式自定义

你可以自定义通知系统的样式：

### NotificationBell
```css
/* 覆盖铃铛样式 */
.notification-bell {
  /* 你的样式 */
}

.notification-bell .badge {
  /* 你的徽章样式 */
}
```

### NotificationCenter
```css
/* 覆盖通知中心样式 */
.notification-center {
  /* 你的样式 */
}
```

### NotificationItem
```css
/* 覆盖通知项样式 */
.notification-item {
  /* 你的样式 */
}
```

## 配置选项

### WebSocket配置
```typescript
// 可以通过环境变量配置
const WS_URL = process.env.REACT_APP_WS_URL || 'ws://localhost:8080/ws';

// 在.env文件中设置
# .env
REACT_APP_WS_URL=ws://your-server.com/ws
```

### 加密配置
```typescript
// websocket.ts中修改
const encryptionEnabled = process.env.REACT_APP_ENCRYPTION_ENABLED === 'true';

// 在.env文件中设置
# .env
REACT_APP_ENCRYPTION_ENABLED=true
```

## 性能优化

### 1. 懒加载
```tsx
const NotificationSystem = React.lazy(() => import('./components/NotificationSystem'));

// 使用Suspense
<Suspense fallback={<div>Loading...</div>}>
  <NotificationSystem token={authToken} />
</Suspense>
```

### 2. 防抖刷新
```typescript
import { debounce } from './utils/debounce';

const fetchUnreadCountDebounced = debounce(fetchUnreadCount, 1000);
```

## 下一步

1. 集成上述代码到你的App.tsx
2. 运行`npm install`安装依赖
3. 测试通知功能
4. 自定义样式和配置
5. 添加到生产环境

如有问题，请查看详细的代码复核报告：`CODE_REVIEW_REPORT.md`
