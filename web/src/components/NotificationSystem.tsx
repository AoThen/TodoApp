import React, { useState, useEffect } from 'react';
import NotificationBell from './notifications/NotificationBell';
import NotificationCenter from './notifications/NotificationCenter';
import { NotificationService } from '../services/notification';
import { websocketService } from '../services/websocket';
import { WSMessage } from '../services/websocket';

interface NotificationSystemProps {
  token: string;
}

const NotificationSystem: React.FC<NotificationSystemProps> = ({ token }) => {
  const [showNotificationCenter, setShowNotificationCenter] = useState(false);
  const [unreadCount, setUnreadCount] = useState(0);
  const [isInitialized, setIsInitialized] = useState(false);

  const notificationService = new NotificationService(
    (window as any).apiService.client
  );

  const fetchUnreadCount = async () => {
    try {
      const response = await notificationService.getUnreadCount();
      setUnreadCount(response.unread_count);
    } catch (err) {
      console.error('Failed to fetch unread count:', err);
    }
  };

  useEffect(() => {
    if (!token) return;

    // 初始化WebSocket连接
    const initWebSocket = async () => {
      try {
        await websocketService.connect(token, true);

        // 注册通知消息处理器
        websocketService.onMessage('notification', handleNotificationMessage);

        setIsInitialized(true);
      } catch (err) {
        console.error('Failed to initialize WebSocket:', err);
      }
    };

    initWebSocket();

    // 初始化时获取未读数量
    fetchUnreadCount();

    // 定期刷新未读数量（每分钟）
    const countInterval = setInterval(fetchUnreadCount, 60000);

    return () => {
      websocketService.offMessage('notification');
      websocketService.disconnect();
      clearInterval(countInterval);
    };
  }, [token]);

  const handleNotificationMessage = (message: WSMessage) => {
    if (message.data) {
      // 更新未读数量
      fetchUnreadCount();

      // 显示 toast 通知（如果安装了 react-toastify）
      if ((window as any).toast) {
        (window as any).toast.info(message.data.title || '新通知', {
          description: message.data.content
        });
      }
    }
  };

  const handleOpenNotificationCenter = () => {
    setShowNotificationCenter(true);
    fetchUnreadCount();
  };

  const handleCloseNotificationCenter = () => {
    setShowNotificationCenter(false);
  };

  return (
    <>
      <NotificationBell
        count={unreadCount}
        onClick={handleOpenNotificationCenter}
      />
      {showNotificationCenter && (
        <>
          <div
            className="notification-overlay"
            onClick={handleCloseNotificationCenter}
          />
          <NotificationCenter
            onClose={handleCloseNotificationCenter}
            onUpdateCount={fetchUnreadCount}
          />
        </>
      )}
    </>
  );
};

export default NotificationSystem;
