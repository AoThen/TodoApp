import React, { useState, useEffect } from 'react';
import { adminService } from '../../services/admin';

interface SystemConfig {
  [key: string]: string;
}

const SystemConfig: React.FC = () => {
  const [config, setConfig] = useState<SystemConfig>({});
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [editConfig, setEditConfig] = useState({ key: '', value: '', description: '' });
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  useEffect(() => {
    loadConfig();
  }, []);

  const loadConfig = async () => {
    try {
      const data = await adminService.getConfig();
      setConfig(data);
    } catch (err) {
      setError('加载配置失败');
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await adminService.setConfig(editConfig.key, editConfig.value, editConfig.description);
      setSuccess('配置已更新');
      setShowModal(false);
      setEditConfig({ key: '', value: '', description: '' });
      loadConfig();
    } catch (err: any) {
      setError(err.response?.data?.error || '保存配置失败');
    }
  };

  const openEditModal = (key: string, value: string, description: string) => {
    setEditConfig({ key, value, description });
    setShowModal(true);
  };

  const configDescriptions: Record<string, string> = {
    max_login_attempts: '最大登录失败尝试次数',
    login_attempt_window_minutes: '登录尝试计数时间窗口（分钟）',
    lockout_duration_minutes: '账户锁定持续时间（分钟）',
    access_token_duration_minutes: '访问令牌有效期（分钟）',
    refresh_token_duration_days: '刷新令牌有效期（天）',
    allow_public_registration: '是否允许公开注册',
  };

  if (loading) {
    return <div className="loading">加载中...</div>;
  }

  return (
    <div>
      <div className="section-header">
        <h2>系统配置</h2>
        <button className="btn btn-primary" onClick={() => { setEditConfig({ key: '', value: '', description: '' }); setShowModal(true); }}>
          添加配置
        </button>
      </div>

      {error && <div className="alert alert-error">{error}</div>}
      {success && <div className="alert alert-success">{success}</div>}

      <div className="config-grid">
        {Object.entries(config).map(([key, value]) => (
          <div key={key} className="config-item">
            <h4>{key}</h4>
            <div className="value">{value}</div>
            <div className="description">{configDescriptions[key] || ''}</div>
            <div style={{ marginTop: '10px' }}>
              <button
                className="btn btn-secondary"
                onClick={() => openEditModal(key, value, configDescriptions[key] || '')}
              >
                编辑
              </button>
            </div>
          </div>
        ))}
        {Object.keys(config).length === 0 && (
          <div style={{ gridColumn: '1 / -1', textAlign: 'center', padding: '20px', color: '#666' }}>
            暂无配置项
          </div>
        )}
      </div>

      {showModal && (
        <div className="modal-overlay" onClick={() => setShowModal(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h2>{editConfig.key ? '编辑配置' : '添加配置'}</h2>
            <form onSubmit={handleSave}>
              {!editConfig.key && (
                <div className="form-group">
                  <label>配置键</label>
                  <input
                    type="text"
                    value={editConfig.key}
                    onChange={(e) => setEditConfig({ ...editConfig, key: e.target.value })}
                    required
                    placeholder="e.g., max_login_attempts"
                  />
                </div>
              )}
              <div className="form-group">
                <label>配置值</label>
                <input
                  type="text"
                  value={editConfig.value}
                  onChange={(e) => setEditConfig({ ...editConfig, value: e.target.value })}
                  required
                />
              </div>
              <div className="form-group">
                <label>描述</label>
                <input
                  type="text"
                  value={editConfig.description}
                  onChange={(e) => setEditConfig({ ...editConfig, description: e.target.value })}
                />
              </div>
              <div className="modal-actions">
                <button type="button" className="btn btn-secondary" onClick={() => setShowModal(false)}>
                  取消
                </button>
                <button type="submit" className="btn btn-primary">
                  保存
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};

export default SystemConfig;
