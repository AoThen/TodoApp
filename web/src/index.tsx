import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import { getInitialTheme, applyTheme } from './utils/theme';

// 主题初始化：需在首帧渲染前执行，避免闪烁
applyTheme(getInitialTheme());

const root = ReactDOM.createRoot(
  document.getElementById('root') as HTMLElement
);

root.render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
