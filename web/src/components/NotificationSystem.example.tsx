import React from 'react';
import NotificationSystem from './NotificationSystem';

// 在 App.tsx 中使用示例：

/*
1. 在你的 App.tsx 顶部导入：
   import NotificationSystem from './components/NotificationSystem';

2. 在渲染的 JSX 中添加（在适当的位置，比如导航栏）：
   {authToken && <NotificationSystem token={authToken} />}

3. 在 App.tsx 的 return 中添加 overlay 样式（如果还没有的话）：
   {showNotificationCenter && (
     <div className="notification-overlay" onClick={() => setShowNotificationCenter(false)} />
   )}

完整示例：
*/

const AppIntegrationExample: React.FC = () => {
  const [authToken, setAuthToken] = useState<string | null>(
    localStorage.getItem('access_token')
  );

  return (
    <div className="app">
      {/* 你的应用头部 */}
      <header className="app-header">
        <h1>TodoApp</h1>
        {/* 其他导航 */}

        {/* 通知系统集成 */}
        {authToken && <NotificationSystem token={authToken} />}
      </header>

      {/* 你的应用主体 */}
      <main className="app-main">
        {/* 你的应用内容 */}
      </main>
    </div>
  );
};

export default AppIntegrationExample;
