import React, { memo, useCallback, useState } from 'react';

interface HeaderProps {
  isOnline: boolean;
  isSyncing: boolean;
  onLogout: () => void;
  onOpenAdmin: () => void;
  onOpenPairing: () => void;
  onOpenImport: () => void;
  isAdmin: boolean;
  bell?: React.ReactNode;
}

const Header: React.FC<HeaderProps> = memo(({ isOnline, isSyncing, onLogout, onOpenAdmin, onOpenPairing, onOpenImport, isAdmin, bell }) => {
  const [theme, setTheme] = useState<'light' | 'dark'>(
    () => (document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light')
  );

  const toggleTheme = useCallback(() => {
    setTheme(prev => {
      const next = prev === 'dark' ? 'light' : 'dark';
      document.documentElement.dataset.theme = next;
      localStorage.setItem('theme', next);
      return next;
    });
  }, []);

  const handleLogout = useCallback(() => {
    onLogout();
  }, [onLogout]);

  return (
    <header className="app-header">
      <div className="header-left">
        <h1>TodoApp</h1>
        <span className={`status-badge ${isOnline ? 'online' : 'offline'}`}>
          {isOnline ? '在线' : '离线'}
        </span>
        {isSyncing && <span className="sync-indicator">同步中...</span>}
      </div>
      <div className="header-actions">
        {bell}
        <button onClick={toggleTheme} className="header-btn" title="切换深浅主题">
          {theme === 'dark' ? '浅色' : '深色'}
        </button>
        <button onClick={onOpenImport} className="header-btn">导入</button>
        <button onClick={onOpenPairing} className="header-btn">设备配对</button>
        {isAdmin && (
          <button onClick={onOpenAdmin} className="header-btn admin-btn">管理</button>
        )}
        <button onClick={handleLogout} className="header-btn logout-btn">登出</button>
      </div>
    </header>
  );
});

Header.displayName = 'Header';

export default Header;
