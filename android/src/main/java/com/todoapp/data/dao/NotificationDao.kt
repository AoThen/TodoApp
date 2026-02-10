package com.todoapp.data.dao

import androidx.room.*
import com.todoapp.data.entities.NotificationEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface NotificationDao {
    
    @Query("SELECT * FROM notifications ORDER BY created_at DESC")
    fun getAllNotifications(): Flow<List<NotificationEntity>>
    
    @Query("SELECT * FROM notifications WHERE is_read = 0 ORDER BY created_at DESC")
    fun getUnreadNotifications(): Flow<List<NotificationEntity>>
    
    @Query("SELECT * FROM notifications WHERE id = :id LIMIT 1")
    suspend fun getNotificationById(id: Long): NotificationEntity?
    
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertNotification(notification: NotificationEntity): Long
    
    @Update
    suspend fun updateNotification(notification: NotificationEntity)
    
    @Query("UPDATE notifications SET is_read = 1, read_at = :readAt WHERE id = :id")
    suspend fun markAsRead(id: Long, readAt: String)
    
    @Query("UPDATE notifications SET is_read = 1, read_at = :readAt")
    suspend fun markAllAsRead(readAt: String)
    
    @Query("DELETE FROM notifications WHERE id = :id")
    suspend fun deleteNotification(id: Long)
    
    @Query("DELETE FROM notifications")
    suspend fun clearAllNotifications()
    
    @Query("SELECT COUNT(*) FROM notifications WHERE is_read = 0")
    suspend fun getUnreadCount(): Int
    
    @Query("SELECT COUNT(*) FROM notifications")
    suspend fun getTotalCount(): Int
}