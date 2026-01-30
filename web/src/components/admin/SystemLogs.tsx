import React, { useState, useEffect } from 'react';
import { adminService, LoginLog, ActionLog } from '../../services/admin';

const SystemLogs: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'login' | 'actions'>('login');
  const [loginLogs, setLoginLogs] = useState<LoginLog[]>([]);
  const [actionLogs, setActionLogs] = useState<ActionLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [filters, setFilters] = useState({
    email: '',
    action: '',
    success: '',
    start_time: '',
    end_time: '',
  });

  useEffect(() => {
    loadLogs();
  }, [activeTab, filters]);

  const loadLogs = async () => {
    setLoading(true);
    try {
      if (activeTab === 'login') {
        const data = await adminService.getLoginLogs({
          email: filters.email || undefined,
          success: filters.success ? filters.success === 'true' : undefined,
          start_time: filters.start_time || undefined,
          end_time: filters.end_time || undefined,
        });
        setLoginLogs(data);
      } else {
        const data = await adminService.getActionLogs({
          email: filters.email || undefined,
          action: filters.action || undefined,
          start_time: filters.start_time || undefined,
          end_time: filters.end_time || undefined,
        });
        setActionLogs(data);
      }
    } catch (err) {
      console.error('Failed to load logs:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleFilterChange = (key: string, value: string) => {
    setFilters({ ...filters, [key]: value });
  };

  return (
    <div>
      <div className="section-header">
        <h2>系统日志</h2>
      </div>

      <div className="admin-nav" style={{ marginBottom: '20px' }}>
        <button
          className={activeTab === 'login' ? 'active' : ''}
          onClick={() => setActiveTab('login')}
        >
          登录日志
        </button>
        <button
          className={activeTab === 'actions' ? 'active' : ''}
          onClick={() => setActiveTab('actions')}
        >
          操作日志
        </button>
      </div>

      <div className="filter-bar">
        <input
          type="text"
          placeholder="邮箱搜索"
          value={filters.email}
          onChange={(e) => handleFilterChange('email', e.target.value)}
        />
        {activeTab === 'login' && (
          <select
            value={filters.success}
            onChange={(e) => handleFilterChange('success', e.target.value)}
          >
            <option value="">全部状态</option>
            <option value="true">成功</option>
            <option value="false">失败</option>
          </select>
        )}
        {activeTab === 'actions' && (
          <select
            value={filters.action}
            onChange={(e) => handleFilterChange('action', e.target.value)}
          >
            <option value="">全部操作</option>
            <option value="create_user">创建用户</option>
            <option value="update_user">更新用户</option>
            <option value="delete_user">删除用户</option>
            <option value="reset_password">重置密码</option>
            <option value="lock_user">锁定用户</option>
            <option value="unlock_user">解锁用户</option>
            <option value="update_config">更新配置</option>
          </select>
        )}
        <input
          type="datetime-local"
          placeholder="开始时间"
          value={filters.start_time}
          onChange={(e) => handleFilterChange('start_time', e.target.value)}
        />
        <input
          type="datetime-local"
          placeholder="结束时间"
          value={filters.end_time}
          onChange={(e) => handleFilterChange('end_time', e.target.value)}
        />
        <button
          className="btn btn-secondary"
          onClick={() => setFilters({ email: '', action: '', success: '', start_time: '', end_time: '' })}
        >
          重置
        </button>
      </div>

      {loading ? (
        <div className="loading">加载中...</div>
      ) : (
        <table className="admin-table">
          <thead>
            <tr>
              {activeTab === 'login' ? (
                <>
                  <th>ID</th>
                  <th>邮箱</th>
                  <th>IP 地址</th>
                  <th>状态</th>
                  <th>时间</th>
                </>
              ) : (
                <>
                  <th>ID</th>
                  <th>管理员</th>
                  <th>操作</th>
                  <th>目标用户</th>
                  <th>详情</th>
                  <th>IP 地址</th>
                  <th>时间</th>
                </>
              )}
            </tr>
          </thead>
          <tbody>
            {activeTab === 'login' && loginLogs.map((log) => (
              <tr key={log.id}>
                <td>{log.id}</td>
                <td>{log.email}</td>
                <td>{log.ip_address}</td>
                <td>
                  <span className={`badge ${log.success ? 'badge-active' : 'badge-locked'}`}>
                    {log.success ? '成功' : '失败'}
                  </span>
                </td>
                <td>{new Date(log.timestamp).toLocaleString()}</td>
              </tr>
            ))}
            {activeTab === 'actions' && actionLogs.map((log) => (
              <tr key={log.id}>
                <td>{log.id}</td>
                <td>{log.admin_email}</td>
                <td>{getActionLabel(log.action)}</td>
                <td>{log.target_email || '-'}</td>
                <td style={{ maxWidth: '200px', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {log.details}
                </td>
                <td>{log.ip_address}</td>
                <td>{new Date(log.timestamp).toLocaleString()}</td>
              </tr>
            ))}
            {((activeTab === 'login' && loginLogs.length === 0) ||
              (activeTab === 'actions' && actionLogs.length === 0)) && (
              <tr>
                <td colSpan={7} style={{ textAlign: 'center', padding: '20px', color: '#666' }}>
                  暂无日志记录
                </td>
              </tr>
            )}
          </tbody>
        </table>
      )}
    </div>
  );
};

function getActionLabel(action: string): string {
  const labels: Record<string, string> = {
    create_user: '创建用户',
    update_user: '更新用户',
    delete_user: '删除用户',
    reset_password: '重置密码',
    lock_user: '锁定用户',
    unlock_user: '解锁用户',
    update_config: '更新配置',
  };
  return labels[action] || action;
}

export default SystemLogs;
