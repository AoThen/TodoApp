import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';

// 主题初始化：localStorage 优先，其次跟随系统（需在首帧渲染前执行，避免闪烁）
const savedTheme = localStorage.getItem('theme');
const initialTheme =
  savedTheme === 'dark' || savedTheme === 'light'
    ? savedTheme
    : window.matchMedia('(prefers-color-scheme: dark)').matches
      ? 'dark'
      : 'light';
document.documentElement.dataset.theme = initialTheme;

const root = ReactDOM.createRoot(
  document.getElementById('root') as HTMLElement
);

root.render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
