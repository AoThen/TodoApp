import { AxiosInstance } from 'axios';

export interface Notification {
  id: number;
  user_id: number;
  type: string;
  title: string;
  content: string;
  priority: 'urgent' | 'high' | 'normal' | 'low';
  is_read: boolean;
  read_at: string | null;
  created_at: string;
  expires_at: string | null;
}

export interface NotificationFilters {
  read?: string;
  type?: string;
  priority?: string;
}

export interface NotificationCreateRequest {
  type: string;
  title: string;
  content: string;
  priority?: 'urgent' | 'high' | 'normal' | 'low';
  expires_at?: string;
}

export interface NotificationListResponse {
  notifications: Notification[];
  pagination: {
    page: number;
    page_size: number;
    total: number;
    pages: number;
  };
}

export class NotificationService {
  constructor(private apiClient: AxiosInstance) {}

  async getNotifications(
    page: number = 1,
    pageSize: number = 20,
    filters: NotificationFilters = {}
  ): Promise<NotificationListResponse> {
    const params = new URLSearchParams();
    params.append('page', page.toString());
    params.append('page_size', pageSize.toString());
    if (filters.read !== undefined) params.append('read', filters.read);
    if (filters.type !== undefined) params.append('type', filters.type);
    if (filters.priority !== undefined) params.append('priority', filters.priority);

    const response = await this.apiClient.get<NotificationListResponse>(
      `/api/v1/notifications?${params.toString()}`
    );
    return response.data;
  }

  async createNotification(data: NotificationCreateRequest): Promise<Notification> {
    const response = await this.apiClient.post<Notification>(
      '/api/v1/notifications',
      data
    );
    return response.data;
  }

  async markAsRead(notificationId: number): Promise<void> {
    await this.apiClient.patch(`/api/v1/notifications/${notificationId}/read`);
  }

  async markAllAsRead(): Promise<{ status: string; marked_count: number }> {
    const response = await this.apiClient.patch('/api/v1/notifications/read-all');
    return response.data;
  }

  async deleteNotification(notificationId: number): Promise<void> {
    await this.apiClient.delete(`/api/v1/notifications/${notificationId}`);
  }

  async clearNotifications(olderThanDays: number = 30): Promise<{ status: string; cleared_count: number }> {
    const response = await this.apiClient.delete(`/api/v1/notifications/clear?older_than_days=${olderThanDays}`);
    return response.data;
  }

  async getUnreadCount(types?: string[]): Promise<{ unread_count: number }> {
    let url = '/api/v1/notifications/unread-count';
    if (types && types.length > 0) {
      url += `?type=${types.join(',')}`;
    }
    const response = await this.apiClient.get<{ unread_count: number }>(url);
    return response.data;
  }
}
