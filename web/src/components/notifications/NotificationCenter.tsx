import React, { useState, useEffect } from 'react';
import { Notification, NotificationService, NotificationFilters } from '../../services/notification';
import NotificationItem from './NotificationItem';
import './NotificationCenter.css';

interface NotificationCenterProps {
  onClose: () => void;
  onUpdateCount: () => void;
}

const NotificationCenter: React.FC<NotificationCenterProps> = ({ onClose, onUpdateCount }) => {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [pagination, setPagination] = useState({ page: 1, page_size: 20, pages: 0 });
  const [filters, setFilters] = useState<NotificationFilters>({});
  const [error, setError] = useState<string | null>(null);

  const notificationService = new NotificationService(
    (window as any).apiService.client
  );

  const loadNotifications = async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await notificationService.getNotifications(page, 20, filters);
      setNotifications(response.notifications);
      setTotal(response.pagination.total);
      setPagination(response.pagination);
      onUpdateCount();
    } catch (err) {
      setError('加载通知失败');
      console.error('Failed to load notifications:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadNotifications();
  }, [page, filters]);

  const handleMarkAsRead = async (id: number) => {
    try {
      await notificationService.markAsRead(id);
      setNotifications(prev =>
        prev.map(notif =>
          notif.id === id ? { ...notif, is_read: true, read_at: new Date().toISOString() } : notif
        )
      );
      onUpdateCount();
    } catch (err) {
      console.error('Failed to mark notification as read:', err);
    }
  };

  const handleDelete = async (id: number) => {
    if (!window.confirm('确认删除此通知？')) return;

    try {
      await notificationService.deleteNotification(id);
      setNotifications(prev => prev.filter(notif => notif.id !== id));
      setTotal(prev => prev - 1);
      onUpdateCount();
    } catch (err) {
      console.error('Failed to delete notification:', err);
    }
  };

  const handleMarkAllAsRead = async () => {
    try {
      await notificationService.markAllAsRead();
      setNotifications(prev =>
        prev.map(notif => ({
          ...notif,
          is_read: true,
          read_at: new Date().toISOString()
        }))
      );
      onUpdateCount();
    } catch (err) {
      console.error('Failed to mark all as read:', err);
    }
  };

  const handleClearAll = async () => {
    if (!window.confirm('确认清空所有通知？')) return;

    try {
      await notificationService.clearNotifications(0);
      setNotifications([]);
      setTotal(0);
      onUpdateCount();
    } catch (err) {
      console.error('Failed to clear notifications:', err);
    }
  };

  const handleFilterChange = (key: keyof NotificationFilters, value: string) => {
    setFilters(prev => ({ ...prev, [key]: value || undefined }));
    setPage(1);
  };

  const getFilterLabel = (key: keyof NotificationFilters): string => {
    switch (key) {
      case 'read':
        return '已读状态';
      case 'type':
        return '通知类型';
      case 'priority':
        return '优先级';
      default:
        return '';
    }
  };

  return (
    <div className="notification-center">
      <div className="notification-header">
        <h2>通知中心</h2>
        <button className="close-button" onClick={onClose} title="关闭">
          ✕
        </button>
      </div>

      <div className="notification-filters">
        <div className="filter-group">
          <select
            value={filters.read || ''}
            onChange={(e) => handleFilterChange('read', e.target.value)}
          >
            <option value="">全部</option>
            <option value="false">未读</option>
            <option value="true">已读</option>
          </select>

          <select
            value={filters.priority || ''}
            onChange={(e) => handleFilterChange('priority', e.target.value)}
          >
            <option value="">所有优先级</option>
            <option value="urgent">紧急</option>
            <option value="high">高</option>
            <option value="normal">普通</option>
            <option value="low">低</option>
          </select>
        </div>

        <div className="notification-actions">
          {total > 0 && (
            <>
              <button onClick={handleMarkAllAsRead} className="action-button">
                全部已读
              </button>
              <button onClick={handleClearAll} className="action-button danger">
                清空
              </button>
            </>
          )}
        </div>
      </div>

      {error && <div className="error-message">{error}</div>}

      <div className="notification-body">
        {loading ? (
          <div className="loading">加载中...</div>
        ) : notifications.length === 0 ? (
          <div className="empty-state">
            <div className="empty-icon">📬</div>
            <p>暂无通知</p>
          </div>
        ) : (
          <div className="notification-list">
            {notifications.map((notification) => (
              <NotificationItem
                key={notification.id}
                notification={notification}
                onMarkAsRead={handleMarkAsRead}
                onDelete={handleDelete}
              />
            ))}
          </div>
        )}
      </div>

      {pagination.pages > 1 && (
        <div className="notification-pagination">
          <button
            onClick={() => setPage(p => p - 1)}
            disabled={page === 1}
            className="pagination-button"
          >
            上一页
          </button>
          <span className="page-info">
            第 {page} / {pagination.pages} 页 ({total} 条)
          </span>
          <button
            onClick={() => setPage(p => p + 1)}
            disabled={page === pagination.pages}
            className="pagination-button"
          >
            下一页
          </button>
        </div>
      )}
    </div>
  );
};

export default NotificationCenter;
