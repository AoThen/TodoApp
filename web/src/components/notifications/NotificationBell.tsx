import React from 'react';
import './NotificationBell.css';

interface NotificationBellProps {
  count: number;
  onClick: () => void;
}

const NotificationBell: React.FC<NotificationBellProps> = ({ count, onClick }) => {
  return (
    <div className="notification-bell" onClick={onClick} title="通知中心">
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path
          d="M12 22C13.6569 22 15 20.6569 15 19H9C9 20.6569 10.3431 22 12 22ZM18 16V11C18 7.93 16.36 5.36 13.5 4.68V4C13.5 3.17 12.83 2.5 12 2.5C11.17 2.5 10.5 3.17 10.5 4V4.68C7.63 5.36 6 7.92 6 11V16L4 18V19H20V18L18 16Z"
          fill="currentColor"
        />
      </svg>
      {count > 0 && <span className="badge">{count > 99 ? '99+' : count}</span>}
    </div>
  );
};

export default NotificationBell;
