import { AxiosInstance } from 'axios';
import { apiService } from './api';

class AdminService {
  private client: AxiosInstance;

  constructor() {
    // 复用主客户端：鉴权头、X-Request-ID、超时、401 自动续期与 {success,data} 解包统一由 api.ts 处理
    this.client = apiService.getClient();
  }

  async getUsers(params?: { page?: number; page_size?: number; email?: string; role?: string }): Promise<{ users: User[]; pagination: Pagination }> {
    const queryParams = new URLSearchParams()
    if (params) {
      if (params.page !== undefined) queryParams.append('page', String(params.page))
      if (params.page_size !== undefined) queryParams.append('page_size', String(params.page_size))
      if (params.email) queryParams.append('email', params.email)
      if (params.role) queryParams.append('role', params.role)
    }
    const response = await this.client.get(`/admin/users?${queryParams}`)
    return response.data
  }

  async createUser(data: CreateUserRequest): Promise<{ id: number; email: string; role: string }> {
    const response = await this.client.post('/admin/users', data);
    return response.data;
  }

  async updateUser(id: number, data: UpdateUserRequest): Promise<void> {
    await this.client.patch(`/admin/users/${id}`, data);
  }

  async resetPassword(id: number, newPassword: string): Promise<void> {
    await this.client.post(`/admin/users/${id}/password`, { new_password: newPassword });
  }

  async lockUser(id: number, durationMinutes: number = 30): Promise<void> {
    await this.client.post(`/admin/users/${id}/lock`, { duration_minutes: durationMinutes });
  }

  async unlockUser(id: number): Promise<void> {
    await this.client.post(`/admin/users/${id}/unlock`);
  }

  async deleteUser(id: number): Promise<void> {
    await this.client.delete(`/admin/users/${id}`);
  }

  async getLoginLogs(filters?: LogFilters): Promise<LoginLog[]> {
    const params = new URLSearchParams();
    if (filters) {
      if (filters.email) params.append('email', filters.email);
      if (filters.success !== undefined) params.append('success', String(filters.success));
      if (filters.start_time) params.append('start_time', filters.start_time);
      if (filters.end_time) params.append('end_time', filters.end_time);
      if (filters.page) params.append('page', String(filters.page));
      if (filters.page_size) params.append('page_size', String(filters.page_size));
    }
    const response = await this.client.get(`/admin/logs/login?${params.toString()}`);
    return response.data;
  }

  async getActionLogs(filters?: LogFilters): Promise<ActionLog[]> {
    const params = new URLSearchParams();
    if (filters) {
      if (filters.email) params.append('email', filters.email);
      if (filters.target_email) params.append('target_email', filters.target_email);
      if (filters.action) params.append('action', filters.action);
      if (filters.start_time) params.append('start_time', filters.start_time);
      if (filters.end_time) params.append('end_time', filters.end_time);
      if (filters.page) params.append('page', String(filters.page));
      if (filters.page_size) params.append('page_size', String(filters.page_size));
    }
    const response = await this.client.get(`/admin/logs/actions?${params.toString()}`);
    return response.data;
  }

  async getConfig(): Promise<Record<string, string>> {
    const response = await this.client.get('/admin/config');
    return response.data;
  }

  async setConfig(key: string, value: string, description?: string): Promise<void> {
    await this.client.put('/admin/config', { key, value, description });
  }
}

export interface Pagination {
  page: number;
  page_size: number;
  total: number;
  pages: number;
  has_prev: boolean;
  has_next: boolean;
}

export interface User {
  id: number;
  email: string;
  role: 'admin' | 'user';
  failed_attempts: number;
  locked_until: string | null;
  must_change_password: boolean;
  created_at: string;
  updated_at: string;
  is_locked: boolean;
}

export interface CreateUserRequest {
  email: string;
  password: string;
  role: 'admin' | 'user';
}

export interface UpdateUserRequest {
  email?: string;
  role?: 'admin' | 'user';
}

export interface LogFilters {
  email?: string;
  target_email?: string;
  action?: string;
  success?: boolean;
  start_time?: string;
  end_time?: string;
  page?: number;
  page_size?: number;
}

export interface LoginLog {
  id: number;
  email: string;
  ip_address: string;
  success: boolean;
  timestamp: string;
}

export interface ActionLog {
  id: number;
  admin_id: number;
  admin_email: string;
  action: string;
  target_user_id: number;
  target_email: string;
  details: string;
  ip_address: string;
  timestamp: string;
}

export const adminService = new AdminService();
