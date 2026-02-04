import React, { useState, useEffect } from 'react';
import { adminService, User } from '../../services/admin';

const UserManagement: React.FC = () => {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState<'create' | 'edit' | 'password' | null>(null);
  const [selectedUser, setSelectedUser] = useState<User | null>(null);
  const [formData, setFormData] = useState({
    email: '',
    password: '',
    role: 'user' as 'admin' | 'user',
  });
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // 分页状态
  const [pagination, setPagination] = useState({
    page: 1,
    pageSize: 20,
    total: 0,
    pages: 0,
    hasPrev: false,
    hasNext: false,
  });
  const [filters, setFilters] = useState({
    email: '',
    role: '',
  });

  useEffect(() => {
    loadUsers();
  }, [pagination.page, filters.email, filters.role]);

  const loadUsers = async () => {
    setLoading(true);
    try {
      const data = await adminService.getUsers({
        page: pagination.page,
        page_size: pagination.pageSize,
        email: filters.email,
        role: filters.role,
      });
      setUsers(data.users);
      setPagination(data.pagination);
      setError(null);
    } catch (err) {
      setError('加载用户列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handlePageChange = (newPage: number) => {
    setPagination(prev => ({ ...prev, page: newPage }));
  };

  const handleFilterChange = (field: 'email' | 'role', value: string) => {
    setFilters(prev => ({ ...prev, [field]: value }));
    setPagination(prev => ({ ...prev, page: 1 })); // 重置到第一页
  };

  const handleClearFilters = () => {
    setFilters({ email: '', role: '' });
    setPagination(prev => ({ ...prev, page: 1 }));
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await adminService.createUser(formData);
      setSuccess('用户创建成功');
      setShowModal(null);
      setFormData({ email: '', password: '', role: 'user' });
      loadUsers();
    } catch (err: any) {
      setError(err.response?.data?.error || '创建用户失败');
    }
  };

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedUser) return;
    try {
      await adminService.updateUser(selectedUser.id, {
        email: formData.email,
        role: formData.role,
      });
      setSuccess('用户更新成功');
      setShowModal(null);
      setSelectedUser(null);
      loadUsers();
    } catch (err: any) {
      setError(err.response?.data?.error || '更新用户失败');
    }
  };

  const handleResetPassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedUser) return;
    try {
      await adminService.resetPassword(selectedUser.id, formData.password);
      setSuccess('密码重置成功');
      setShowModal(null);
      setSelectedUser(null);
      setFormData({ email: '', password: '', role: 'user' });
    } catch (err: any) {
      setError(err.response?.data?.error || '重置密码失败');
    }
  };

  const handleLock = async (user: User) => {
    try {
      await adminService.lockUser(user.id);
      setSuccess(`用户 ${user.email} 已锁定`);
      loadUsers();
    } catch (err: any) {
      setError(err.response?.data?.error || '锁定用户失败');
    }
  };

  const handleUnlock = async (user: User) => {
    try {
      await adminService.unlockUser(user.id);
      setSuccess(`用户 ${user.email} 已解锁`);
      loadUsers();
    } catch (err: any) {
      setError(err.response?.data?.error || '解锁用户失败');
    }
  };

  const handleDelete = async (user: User) => {
    if (!confirm(`确定要删除用户 ${user.email} 吗？`)) return;
    try {
      await adminService.deleteUser(user.id);
      setSuccess('用户已删除');
      loadUsers();
    } catch (err: any) {
      setError(err.response?.data?.error || '删除用户失败');
    }
  };

  const openEditModal = (user: User) => {
    setSelectedUser(user);
    setFormData({ email: user.email, password: '', role: user.role });
    setShowModal('edit');
  };

  const openPasswordModal = (user: User) => {
    setSelectedUser(user);
    setFormData({ email: '', password: '', role: 'user' });
    setShowModal('password');
  };

  if (loading) {
    return <div className="loading">加载中...</div>;
  }

  return (
    <div>
      <div className="section-header">
        <h2>用户管理</h2>
        <button className="btn btn-primary" onClick={() => { setShowModal('create'); setFormData({ email: '', password: '', role: 'user' }); setShowModal(prev => prev || 'create'); setSuccess(null); setError(null); }}>
          创建用户
        </button>
      </div>

      {/* 过滤器 */}
      <div className="admin-filters">
        <input
          type="text"
          placeholder="搜索邮箱"
          value={filters.email}
          onChange={(e) => { setFilters(f => ({...f, email: e.target.value})); setPagination(p => ({...p, page: 1})); setError(null); success && setSuccess(null); }}
          disabled={loading}
        />
        <select
          value={filters.role}
          onChange={(e) => handleFilterChange('role', e.target.value)}
          disabled={loading}
        >
          <option value="">全部角色</option>
          <option value="admin">管理员</option>
          <option value="user">普通用户</option>
        </select>
        {(filters.email || filters.role) && (
          <button className="btn btn-secondary btn-small" onClick={() => { setFilters({ email: '', role: '' }); setPagination(p => ({...p, page: 1})); setError(null); success && setSuccess(null); }}>
            清除筛选
          </button>
        )}
      </div>

      {error && <div className="alert alert-error">{error}</div>}
      {success && <div className="alert alert-success">{success}</div>}

      <table className="admin-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>邮箱</th>
            <th>角色</th>
            <th>状态</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {users.map((user) => (
            <tr key={user.id}>
              <td>{user.id}</td>
              <td>{user.email}</td>
              <td>
                <span className={`badge ${user.role === 'admin' ? 'badge-admin' : 'badge-user'}`}>
                  {user.role === 'admin' ? '管理员' : '普通用户'}
                </span>
              </td>
              <td>
                <span className={`badge ${user.is_locked ? 'badge-locked' : 'badge-active'}`}>
                  {user.is_locked ? '已锁定' : '正常'}
                </span>
              </td>
              <td>{new Date(user.created_at).toLocaleString()}</td>
              <td>
                <button className="btn btn-secondary" onClick={() => openEditModal(user)}>编辑</button>
                <button className="btn btn-primary" onClick={() => openPasswordModal(user)}>重置密码</button>
                {user.is_locked ? (
                  <button className="btn btn-success" onClick={() => handleUnlock(user)}>解锁</button>
                ) : (
                  <button className="btn btn-secondary" onClick={() => handleLock(user)}>锁定</button>
                )}
                <button className="btn btn-danger" onClick={() => handleDelete(user)}>删除</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {/* 分页控件 */}
      {pagination.pages > 1 && (
        <div className="pagination">
          <span>第 {pagination.page} / 共 {pagination.pages} 页 ({pagination.total} 条)</span>
          <div className="pagination-buttons">
            <button
              disabled={!pagination.hasPrev || loading}
              onClick={() => handlePageChange(pagination.page - 1)}
            >
              上一页
            </button>
            <button
              disabled={!pagination.hasNext || loading}
              onClick={() => handlePageChange(pagination.page + 1)}
            >
              下一页
            </button>
          </div>
        </div>
      )}

      {showModal && (
        <div className="modal-overlay" onClick={() => { setShowModal(null); setSelectedUser(null); }}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h2>
              {showModal === 'create' && '创建用户'}
              {showModal === 'edit' && '编辑用户'}
              {showModal === 'password' && `重置密码 - ${selectedUser?.email}`}
            </h2>
            <form onSubmit={showModal === 'password' ? handleResetPassword : showModal === 'create' ? handleCreate : handleUpdate}>
              {showModal !== 'password' && (
                <>
                  <div className="form-group">
                    <label>邮箱</label>
                    <input
                      type="email"
                      value={formData.email}
                      onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                      required
                    />
                  </div>
                  <div className="form-group">
                    <label>角色</label>
                    <select
                      value={formData.role}
                      onChange={(e) => setFormData({ ...formData, role: e.target.value as 'admin' | 'user' })}
                    >
                      <option value="user">普通用户</option>
                      <option value="admin">管理员</option>
                    </select>
                  </div>
                </>
              )}
              {showModal !== 'edit' && (
                <div className="form-group">
                  <label>{showModal === 'password' ? '新密码' : '密码'}</label>
                  <input
                    type="password"
                    value={formData.password}
                    onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                    required={true}
                    minLength={8}
                  />
                </div>
              )}
              <div className="modal-actions">
                <button type="button" className="btn btn-secondary" onClick={() => { setShowModal(null); setSelectedUser(null); }}>
                  取消
                </button>
                <button type="submit" className="btn btn-primary">
                  {showModal === 'create' && '创建'}
                  {showModal === 'edit' && '保存'}
                  {showModal === 'password' && '重置密码'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
export default UserManagement;

