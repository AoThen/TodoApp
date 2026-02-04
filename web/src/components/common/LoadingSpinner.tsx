import React from 'react';

interface LoadingSpinnerProps {
  size?: 'small' | 'medium' | 'large';
  color?: 'primary' | 'success' | 'danger' | 'warning';
  fullScreen?: boolean;
  text?: string;
}

const LoadingSpinner: React.FC<LoadingSpinnerProps> = ({
  size = 'medium',
  color = 'primary',
  fullScreen = false,
  text
}) => {
  const sizeClasses = {
    small: 'w-4 h-4',
    medium: 'w-8 h-8',
    large: 'w-12 h-12'
  };

  const colorClasses = {
    primary: 'border-primary-600',
    success: 'border-green-600',
    danger: 'border-red-600',
    warning: 'border-yellow-600'
  };

  const spinner = (
    <div className="flex flex-col items-center justify-center gap-3">
      <div
        className={`${sizeClasses[size]} ${colorClasses[color]} border-4 border-t-transparent border-solid rounded-full animate-spin`}
        role="status"
        aria-label="Loading"
      />
      {text && <p className={`text-sm ${color === 'primary' ? 'text-gray-600' : colorClasses[color].replace('border-', 'text-')}`}>{text}</p>}
    </div>
  );

  if (fullScreen) {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-white/80 backdrop-blur-sm">
        {spinner}
      </div>
    );
  }

  return (
    <div className="inline-flex" role="status" aria-label="Loading">
      {spinner}
    </div>
  );
};

export default LoadingSpinner;
