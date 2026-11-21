import React, { useState } from 'react';
import { apiService } from '../../services/api';

interface LoginPageProps {
  onSuccess: (token: string) => void | Promise<void>;
}

const LoginPage: React.FC<LoginPageProps> = ({ onSuccess }) => {
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const form = e.currentTarget as HTMLFormElement;
    const email = (form.elements.namedItem('email') as HTMLInputElement).value;
    const password = (form.elements.namedItem('password') as HTMLInputElement).value;

    setSubmitting(true);
    setError('');
    try {
      const { access_token } = await apiService.login(email, password);
      localStorage.setItem('user_email', email);
      await onSuccess(access_token);
    } catch (err) {
      console.error('Login failed:', err);
      setError('登录失败，请检查邮箱和密码');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="login-container">
      <h1>TodoApp Login</h1>
      <form onSubmit={handleSubmit}>
        <input type="email" name="email" placeholder="Email" required />
        <input type="password" name="password" placeholder="Password" required />
        {error && (
          <p className="login-error" role="alert">{error}</p>
        )}
        <button type="submit" disabled={submitting}>
          {submitting ? '登录中...' : 'Login'}
        </button>
      </form>
    </div>
  );
};

export default LoginPage;
