import React, { memo, useCallback } from 'react';

interface HeaderProps {
  isOnline: boolean;
  isSyncing: boolean;
  onLogout: () => void;
  onOpenAdmin: () => void;
  onOpenPairing: () => void;
  onOpenImport: () => void;
  isAdmin: boolean;
}

const Header: React.FC<HeaderProps> = memo(({ isOnline, isSyncing, onLogout, onOpenAdmin, onOpenPairing, onOpenImport, isAdmin }) => {
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
