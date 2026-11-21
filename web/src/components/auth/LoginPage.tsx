import React from 'react';
import { apiService } from '../../services/api';

interface LoginPageProps {
  onSuccess: (token: string) => void | Promise<void>;
}

const LoginPage: React.FC<LoginPageProps> = ({ onSuccess }) => {
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const form = e.currentTarget as HTMLFormElement;
    const email = (form.elements.namedItem('email') as HTMLInputElement).value;
    const password = (form.elements.namedItem('password') as HTMLInputElement).value;

    try {
      const { access_token } = await apiService.login(email, password);
      localStorage.setItem('user_email', email);
      await onSuccess(access_token);
    } catch (error) {
      console.error('Login failed:', error);
      alert('Login failed. Please check your credentials.');
    }
  };

  return (
    <div className="login-container">
      <h1>TodoApp Login</h1>
      <form onSubmit={handleSubmit}>
        <input type="email" name="email" placeholder="Email" required />
        <input type="password" name="password" placeholder="Password" required />
        <button type="submit">Login</button>
      </form>
    </div>
  );
};

export default LoginPage;
