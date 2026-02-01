package com.todoapp.data.notify

import android.content.Context
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import com.todoapp.R
import com.todoapp.data.local.AppDatabase
import com.todoapp.data.local.Notification as NotificationEntity
import kotlinx.coroutines.*
import java.text.SimpleDateFormat
import java.util.*

class NotificationManager(
    private val context: Context,
    private val userId: Int
) {
    private val database = AppDatabase.getInstance(context)
    private val notificationManager = NotificationManagerCompat.from(context)
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())

    private object Config {
        val CHANNEL_ID = "todoapp_notifications"
        val CHANNEL_NAME = "TodoApp 通知"
        val CHANNEL_DESC = "TodoApp 系统通知"
    }

    init {
        createNotificationChannel()
    }

    private fun createNotificationChannel() {
        if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.O) {
            val channel = android.app.NotificationChannel(
                Config.CHANNEL_ID,
                Config.CHANNEL_NAME,
                android.app.NotificationManager.IMPORTANCE_HIGH
            ).apply {
                description = Config.CHANNEL_DESC
                enableVibration(true)
                enableLights(true)
            }

            val notificationManager = context.getSystemService(android.app.NotificationManager::class.java)
            notificationManager.createNotificationChannel(channel)
        }
    }

    /**
     * 创建并保存通知
     */
    suspend fun createNotification(
        type: String,
        title: String,
        content: String,
        priority: String = "normal"
    ): Long = withContext(Dispatchers.IO) {
        val notification = NotificationEntity(
            userId = userId,
            type = type,
            title = title,
            content = content,
            priority = priority,
            createdAt = SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.getDefault())
                .format(Date()),
            expiresAt = null
        )

        val id = database.notificationDao().insertNotification(notification)

        // 发送系统通知
        showSystemNotification(id.toInt(), title, content, priority)

        return@withContext id
    }

    /**
     * 显示系统通知
     */
    private fun showSystemNotification(id: Int, title: String, content: String, priority: String) {
        val importance = when (priority) {
            "urgent", "high" -> NotificationCompat.PRIORITY_HIGH
            "normal" -> NotificationCompat.PRIORITY_DEFAULT
            "low" -> NotificationCompat.PRIORITY_LOW
            else -> NotificationCompat.PRIORITY_DEFAULT
        }

        val notification = NotificationCompat.Builder(context, Config.CHANNEL_ID)
            .setContentTitle(title)
            .setContentText(content)
            .setSmallIcon(R.drawable.ic_launcher_foreground)
            .setPriority(importance)
            .setAutoCancel(true)
            .build()

        notificationManager.notify(id, notification)
    }

    /**
     * 获取通知列表（分页）
     */
    suspend fun getNotifications(page: Int, pageSize: Int): List<NotificationEntity> =
        withContext(Dispatchers.IO) {
            val offset = (page - 1) * pageSize
            database.notificationDao().getNotificationsPaginated(userId, pageSize, offset)
        }

    /**
     * 获取未读数量
     */
    suspend fun getUnreadCount(types: List<String>? = null): Int = withContext(Dispatchers.IO) {
        if (types.isNullOrEmpty()) {
            database.notificationDao().getUnreadCount(userId)
        } else {
            database.notificationDao().getUnreadCountByType(userId, types)
        }
    }

    /**
     * 标记为已读
     */
    suspend fun markAsRead(notificationId: Long): Boolean = withContext(Dispatchers.IO) {
        try {
            val notification = database.notificationDao().getNotificationById(notificationId)
            if (notification != null) {
                val readAt = SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.getDefault())
                    .format(Date())
                database.notificationDao().markAsRead(notificationId, readAt)
                // 取消系统通知
                notificationManager.cancel(notificationId.toInt())
                return@withContext true
            }
            false
        } catch (e: Exception) {
            android.util.Log.e("NotificationManager", "Failed to mark as read", e)
            false
        }
    }

    /**
     * 标记全部已读
     */
    suspend fun markAllAsRead(): Int = withContext(Dispatchers.IO) {
        val readAt = SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.getDefault())
            .format(Date())
        database.notificationDao().markAllAsRead(userId, readAt)
    }

    /**
     * 删除通知
     */
    suspend fun deleteNotification(notificationId: Long): Boolean = withContext(Dispatchers.IO) {
        try {
            database.notificationDao().deleteNotification(notificationId)
            notificationManager.cancel(notificationId.toInt())
            true
        } catch (e: Exception) {
            android.util.Log.e("NotificationManager", "Failed to delete", e)
            false
        }
    }

    /**
     * 清空通知
     */
    suspend fun clearNotifications(olderThanDays: Int? = null): Int = withContext(Dispatchers.IO) {
        if (olderThanDays == null || olderThanDays <= 0) {
            database.notificationDao().clearAllNotifications(userId)
        } else {
            database.notificationDao().clearNotifications(userId, olderThanDays)
        }
    }

    /**
     * 清理过期通知
     */
    suspend fun cleanupExpired(): Int = withContext(Dispatchers.IO) {
        database.notificationDao().deleteExpiredNotifications()
    }

    /**
     * 清理所有
     */
    fun cleanup() {
        scope.cancel()
    }
}
