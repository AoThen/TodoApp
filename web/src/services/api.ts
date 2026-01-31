import axios, { AxiosInstance, AxiosError, AxiosRequestConfig } from 'axios';
import { Task, DeltaChange } from './indexedDB';

const API_BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080/api/v1';

// Request timeout (30 seconds)
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
      withCredentials: true, // Enable credentials for CORS
    });

    // Request interceptor to add auth token
    this.client.interceptors.request.use(
      (config) => {
        const token = localStorage.getItem('access_token');
        if (token) {
          config.headers.Authorization = `Bearer ${token}`;
        }
        // Add request ID for tracing
        config.headers['X-Request-ID'] = this.generateRequestId();
        return config;
      },
      (error) => Promise.reject(error)
    );

    // Response interceptor for token refresh and error handling
    this.client.interceptors.response.use(
      (response) => response,
      async (error: AxiosError) => {
        const originalRequest = error.config as unknown as { _retry?: boolean } & Record<string, unknown>;
        
        // Handle 401 Unauthorized
        if (error.response?.status === 401 && !originalRequest._retry) {
          originalRequest._retry = true;
          
          try {
            // Prevent multiple simultaneous refresh attempts
            if (!this.refreshPromise) {
              this.refreshPromise = this.performTokenRefresh();
            }
            
            await this.refreshPromise;
            this.refreshPromise = null;
            
            // Retry the original request
            return this.client(originalRequest as AxiosRequestConfig);
          } catch (refreshError) {
            this.refreshPromise = null;
            // Clear tokens and redirect to login
            this.clearTokens();
            throw refreshError;
          }
        }

        // Log error for debugging (in production, use proper logging service)
        if (process.env.NODE_ENV === 'development') {
          console.error('API Error:', error.message);
        }

        throw error;
      }
    );
  }

  // Generate unique request ID for tracing
  private generateRequestId(): string {
    return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
  }

  // Clear all stored tokens
  private clearTokens(): void {
    localStorage.removeItem('access_token');
    // Clear any other stored auth data
    sessionStorage.clear();
  }

  // Auth operations
  async login(email: string, password: string): Promise<{
    access_token: string;
    expires_in: number;
  }> {
    try {
      const response = await this.client.post('/auth/login', { email, password });
      const { access_token, expires_in } = response.data;
      
      if (!access_token) {
        throw new Error('No access token received');
      }

      localStorage.setItem('access_token', access_token);
      localStorage.setItem('token_expires_at', String(Date.now() + expires_in * 1000));
      
      return { access_token, expires_in };
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
    try {
      const response = await this.client.post('/auth/refresh');
      const { access_token, expires_in } = response.data;
      
      if (!access_token) {
        throw new Error('No access token received from refresh');
      }

      localStorage.setItem('access_token', access_token);
      localStorage.setItem('token_expires_at', String(Date.now() + expires_in * 1000));
    } catch (error) {
      this.clearTokens();
      throw error;
    }
  }

  async logout(): Promise<void> {
    try {
      await this.client.post('/auth/logout');
    } catch (error) {
      // Log but don't throw - logout should always clear local state
      if (process.env.NODE_ENV === 'development') {
        console.warn('Logout API call failed:', error);
      }
    } finally {
      this.clearTokens();
    }
  }

  // Check if token is expired
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

  async importTasks(data: unknown): Promise<void> {
    await this.client.post('/import', data);
  }

  // User operations
  async getCurrentUser(): Promise<{ id: string; email: string }> {
    const response = await this.client.get('/users/me');
    return response.data;
  }
}

export const apiService = new ApiService();
