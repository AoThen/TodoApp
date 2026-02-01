import React from 'react';
import { Notification } from '../../services/notification';
import './NotificationItem.css';

interface NotificationItemProps {
  notification: Notification;
  onMarkAsRead: (id: number) => void;
  onDelete: (id: number) => void;
}

const NotificationItem: React.FC<NotificationItemProps> = ({ notification, onMarkAsRead, onDelete }) => {
  const getPriorityIcon = (priority: string) => {
    switch (priority) {
      case 'urgent':
        return '🔴';
      case 'high':
        return '🟠';
      case 'normal':
        return '🔵';
      case 'low':
        return '🟢';
      default:
        return '⚪';
    }
  };

  const formattedTime = new Date(notification.created_at).toLocaleString('zh-CN', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  });

  return (
    <div
      className={`notification-item ${notification.is_read ? 'read' : 'unread'}`}
      onClick={() => !notification.is_read && onMarkAsRead(notification.id)}
    >
      <div className="notification-priority">{getPriorityIcon(notification.priority)}</div>
      <div className="notification-content">
        <div className="notification-header">
          <h4 className="notification-title">{notification.title}</h4>
          <span className="notification-time">{formattedTime}</span>
        </div>
        <p className="notification-message">{notification.content}</p>
        {!notification.is_read && <span className="unread-indicator">未读</span>}
      </div>
      <button
        className="notification-delete"
        onClick={(e) => {
          e.stopPropagation();
          onDelete(notification.id);
        }}
        title="删除"
      >
        ✕
      </button>
    </div>
  );
};

export default NotificationItem;
