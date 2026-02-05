import axios, { AxiosInstance, AxiosError, AxiosRequestConfig } from 'axios';
import { Task, DeltaChange } from './indexedDB';

const API_BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080/api/v1';
const REQUEST_TIMEOUT = 30000;

class ApiService {
  private client: AxiosInstance;
  private refreshPromise: Promise<void> | null = null;

  constructor() {
    this.client = axios.create({
      baseURL: API_BASE_URL,
      headers: {
        'Content-Type': 'application/json',
      },
      timeout: REQUEST_TIMEOUT,
      withCredentials: true,
    });

    this.client.interceptors.request.use(
      (config) => {
        const token = this.getAccessToken();
        if (token) {
          config.headers.Authorization = `Bearer ${token}`;
        }
        config.headers['X-Request-ID'] = this.generateRequestId();
        return config;
      },
      (error) => Promise.reject(error)
    );

    this.client.interceptors.response.use(
      (response) => response,
      async (error: AxiosError) => {
        const originalRequest = error.config as unknown as { _retry?: boolean } & Record<string, unknown>;

        if (error.response?.status === 401 && !originalRequest._retry) {
          originalRequest._retry = true;

          try {
            if (!this.refreshPromise) {
              this.refreshPromise = this.performTokenRefresh();
            }

            await this.refreshPromise;
            this.refreshPromise = null;

            return this.client(originalRequest as AxiosRequestConfig);
          } catch (refreshError) {
            this.refreshPromise = null;
            this.clearTokens();
            window.location.href = '/login';
            throw refreshError;
          }
        }

        if (process.env.NODE_ENV === 'development') {
          console.error('API Error:', error.message);
        }

        throw error;
      }
    );
  }

  private generateRequestId(): string {
    return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
  }

  private getAccessToken(): string | null {
    const token = localStorage.getItem('access_token');
    return token || null;
  }

  private getRefreshToken(): string | null {
    return localStorage.getItem('refresh_token');
  }

  private clearTokens(): void {
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
    localStorage.removeItem('token_expires_at');
    sessionStorage.clear();
  }

  getClient(): AxiosInstance {
    return this.client;
  }

  async login(email: string, password: string): Promise<{
    access_token: string;
    refresh_token: string;
    expires_in: number;
  }> {
    try {
      const response = await this.client.post('/auth/login', { email, password });
      const { access_token, refresh_token, expires_in } = response.data;

      if (!access_token || !refresh_token) {
        throw new Error('No tokens received');
      }

      localStorage.setItem('access_token', access_token);
      localStorage.setItem('refresh_token', refresh_token);
      localStorage.setItem('token_expires_at', String(Date.now() + expires_in * 1000));

      return { access_token, refresh_token, expires_in };
    } catch (error) {
      if (process.env.NODE_ENV === 'development') {
        console.error('Login error:', error);
      }
      throw error;
    }
  }

  async refreshToken(): Promise<void> {
    return this.performTokenRefresh();
  }

  private async performTokenRefresh(): Promise<void> {
    const refreshToken = this.getRefreshToken();
    if (!refreshToken) {
      this.clearTokens();
      throw new Error('No refresh token available');
    }

    try {
      const response = await this.client.post('/auth/refresh', { refresh_token: refreshToken });
      const { access_token, refresh_token, expires_in } = response.data;

      if (!access_token) {
        throw new Error('No access token received from refresh');
      }

      localStorage.setItem('access_token', access_token);
      if (refresh_token) {
        localStorage.setItem('refresh_token', refresh_token);
      }
      localStorage.setItem('token_expires_at', String(Date.now() + expires_in * 1000));
    } catch (error) {
      this.clearTokens();
      throw error;
    }
  }

  async logout(): Promise<void> {
    const refreshToken = this.getRefreshToken();
    try {
      await this.client.post('/auth/logout', { refresh_token: refreshToken });
    } catch (error) {
      if (process.env.NODE_ENV === 'development') {
        console.warn('Logout API call failed:', error);
      }
    } finally {
      this.clearTokens();
    }
  }

  isTokenExpired(): boolean {
    const expiresAt = localStorage.getItem('token_expires_at');
    if (!expiresAt) return true;
    return Date.now() > parseInt(expiresAt, 10);
  }

  // Task operations
  async getTasks(): Promise<Task[]> {
    const response = await this.client.get('/tasks');
    return response.data;
  }

  async createTask(task: Partial<Task>): Promise<Task> {
    const response = await this.client.post('/tasks', task);
    return response.data;
  }

  async updateTask(id: number, updates: Partial<Task>): Promise<Task> {
    const response = await this.client.patch(`/tasks/${id}`, updates);
    return response.data;
  }

  async deleteTask(id: number): Promise<void> {
    await this.client.delete(`/tasks/${id}`);
  }

  // Sync operations
  async sync(lastSyncAt: string, changes: DeltaChange[]): Promise<{
    server_changes: Array<{
      id: number;
      server_version: number;
      title: string;
      updated_at: string;
      is_deleted: boolean;
    }>;
    client_changes: Array<{
      local_id: string;
      server_id: number;
      op: string;
    }>;
    last_sync_at: string;
    conflicts: Array<{
      local_id: string;
      server_id: number;
      reason: string;
      options: string[];
    }>;
  }> {
    const response = await this.client.post('/sync', {
      last_sync_at: lastSyncAt,
      changes,
    });
    return response.data;
  }

  // Export operations
  async exportTasks(format: 'json' | 'csv'): Promise<Blob> {
    const response = await this.client.get('/export', {
      params: { type: 'tasks', format },
      responseType: 'blob',
    });
    return response.data;
  }

  async importTasks(data: unknown): Promise<{ imported: number; skipped: number; inserted_ids: number[] }> {
    const response = await this.client.post('/import', data);
    return response.data;
  }

  async batchDeleteTasks(taskIds: number[]) {
    return this.client.delete('/tasks/batch', { data: { task_ids: taskIds } });
  }

  async restoreTask(taskId: number) {
    return this.client.post(`/tasks/${taskId}/restore`);
  }

  // User operations
  async getCurrentUser(): Promise<{ id: string; email: string }> {
    const response = await this.client.get('/users/me');
    return response.data;
  }

  // Device operations
  async getDevices(): Promise<{ data: { devices: Array<{
    id: number;
    device_type: string;
    device_id: string;
    server_url: string;
    paired_at: string;
    last_seen: string;
    is_active: boolean;
  }> } }> {
    const response = await this.client.get('/devices');
    return response.data;
  }

  async pairDevice(data: { key: string; device_type: string; device_id: string }): Promise<void> {
    await this.client.post('/devices/pair', data);
  }

  async regenerateKey(deviceId: string): Promise<{ data: { new_key: string } }> {
    const response = await this.client.post(`/devices/${deviceId}/regenerate`);
    return response.data;
  }

  async revokeDevice(deviceId: string): Promise<void> {
    await this.client.delete(`/devices/${deviceId}`);
  }
}

export const apiService = new ApiService();
