import React, { useState, useEffect } from 'react';
import { adminService, User } from '../../services/admin';
import UserManagement from './UserManagement';
import SystemLogs from './SystemLogs';
import SystemConfig from './SystemConfig';
import './AdminPanel.css';

type TabType = 'users' | 'logs' | 'config';

interface AdminPanelProps {
  onBack: () => void;
}

const AdminPanel: React.FC<AdminPanelProps> = ({ onBack }) => {
  const [activeTab, setActiveTab] = useState<TabType>('users');
  const [isAdmin, setIsAdmin] = useState<boolean>(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    checkAdminStatus();
  }, []);

  const checkAdminStatus = async () => {
    try {
      const users = await adminService.getUsers();
      const currentUserEmail = localStorage.getItem('user_email');
      const currentUser = users.users.find((u: User) => u.email === currentUserEmail);
      
      if (currentUser && currentUser.role === 'admin') {
        setIsAdmin(true);
      } else {
        setError('您没有管理员权限');
      }
    } catch (err) {
      console.error('Failed to check admin status:', err);
      setError('无法验证管理员权限');
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return <div className="admin-loading">加载中...</div>;
  }

  if (error) {
    return <div className="admin-error">{error}</div>;
  }

  return (
    <div className="admin-panel">
      <header className="admin-header">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h1>管理面板</h1>
          <button className="btn btn-secondary" onClick={onBack}>返回应用</button>
        </div>
      </header>

      <nav className="admin-nav">
        <button
          className={activeTab === 'users' ? 'active' : ''}
          onClick={() => setActiveTab('users')}
        >
          用户管理
        </button>
        <button
          className={activeTab === 'logs' ? 'active' : ''}
          onClick={() => setActiveTab('logs')}
        >
          系统日志
        </button>
        <button
          className={activeTab === 'config' ? 'active' : ''}
          onClick={() => setActiveTab('config')}
        >
          系统配置
        </button>
      </nav>

      <main className="admin-content">
        {activeTab === 'users' && <UserManagement />}
        {activeTab === 'logs' && <SystemLogs />}
        {activeTab === 'config' && <SystemConfig />}
      </main>
    </div>
  );
};

export default AdminPanel;
