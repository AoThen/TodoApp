package com.todoapp.data.local

import android.content.Context
import androidx.room.*
import kotlinx.coroutines.flow.Flow

@Entity(tableName = "tasks")
data class Task(
    @PrimaryKey(autoGenerate = true)
    val id: Long = 0,
    val localId: String,
    val serverId: Long? = null,
    val userId: String,
    val serverVersion: Int = 0,
    val title: String,
    val description: String = "",
    val status: String = "todo",
    val priority: String = "medium",
    val dueAt: String? = null,
    val createdAt: String,
    val updatedAt: String,
    val completedAt: String? = null,
    val isDeleted: Boolean = false,
    val lastModified: String
)

@Entity(tableName = "delta_queue")
data class DeltaChange(
    @PrimaryKey(autoGenerate = true)
    val id: Long = 0,
    val localId: String,
    val op: String,
    val payload: String,
    val clientVersion: Int,
    val timestamp: String
)

@Entity(tableName = "sync_meta")
data class SyncMeta(
    @PrimaryKey
    val userId: String,
    val lastSyncAt: String? = null,
    val lastServerVersion: Int = 0
)

@Entity(tableName = "conflicts")
data class Conflict(
    @PrimaryKey(autoGenerate = true)
    val id: Long = 0,
    val localId: String,
    val serverId: Long,
    val reason: String,
    val options: String,
    val createdAt: String
)

@Entity(tableName = "notifications", indices = [
    Index(value = ["userId"]),
    Index(value = ["isRead"]),
    Index(value = ["createdAt"])
])
data class Notification(
    @PrimaryKey(autoGenerate = true)
    val id: Long = 0,
    val userId: Int,
    val type: String,
    val title: String,
    val content: String,
    val priority: String = "normal", // urgent, high, normal, low
    val isRead: Boolean = false,
    val readAt: String? = null,
    val createdAt: String,
    val expiresAt: String? = null
)

@Dao
interface TaskDao {
    @Query("SELECT * FROM tasks WHERE isDeleted = 0 ORDER BY createdAt DESC")
    fun getAllTasks(): Flow<List<Task>>

    @Query("SELECT * FROM tasks WHERE localId = :localId")
    suspend fun getTaskByLocalId(localId: String): Task?

    @Query("SELECT * FROM tasks WHERE id = :id")
    suspend fun getTaskById(id: Long): Task?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertTask(task: Task): Long

    @Update
    suspend fun updateTask(task: Task)

    @Query("UPDATE tasks SET isDeleted = 1, updatedAt = :updatedAt WHERE id = :id")
    suspend fun softDeleteTask(id: Long, updatedAt: String)

    @Query("DELETE FROM tasks")
    suspend fun deleteAllTasks()
}

@Dao
interface DeltaQueueDao {
    @Query("SELECT * FROM delta_queue ORDER BY timestamp ASC")
    suspend fun getAllDeltas(): List<DeltaChange>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertDelta(delta: DeltaChange): Long

    @Query("DELETE FROM delta_queue WHERE id = :id")
    suspend fun deleteDelta(id: Long)

    @Query("DELETE FROM delta_queue")
    suspend fun deleteAllDeltas()
}

@Dao
interface SyncMetaDao {
    @Query("SELECT * FROM sync_meta WHERE userId = :userId")
    suspend fun getSyncMeta(userId: String): SyncMeta?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertSyncMeta(meta: SyncMeta)

    @Query("UPDATE sync_meta SET lastSyncAt = :lastSyncAt WHERE userId = :userId")
    suspend fun updateLastSyncAt(userId: String, lastSyncAt: String)
}

@Dao
interface ConflictDao {
    @Query("SELECT * FROM conflicts")
    suspend fun getAllConflicts(): List<Conflict>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertConflict(conflict: Conflict): Long

    @Query("DELETE FROM conflicts WHERE id = :id")
    suspend fun deleteConflict(id: Long)

    @Query("DELETE FROM conflicts")
    suspend fun deleteAllConflicts()
}

@Dao
interface NotificationDao {
    @Query("SELECT * FROM notifications WHERE userId = :userId ORDER BY createdAt DESC LIMIT :limit OFFSET :offset")
    suspend fun getNotificationsPaginated(userId: Int, limit: Int, offset: Int): List<Notification>

    @Query("SELECT COUNT(*) FROM notifications WHERE userId = :userId AND isRead = 0")
    suspend fun getUnreadCount(userId: Int): Int

    @Query("SELECT * FROM notifications WHERE id = :id")
    suspend fun getNotificationById(id: Long): Notification?

    @Query("SELECT COUNT(*) FROM notifications WHERE userId = :userId AND isRead = 0 AND (:types IS NULL OR type IN (:types))")
    suspend fun getUnreadCountByType(userId: Int, types: List<String>?): Int

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertNotification(notification: Notification): Long

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertNotifications(notifications: List<Notification>): List<Long>

    @Update
    suspend fun updateNotification(notification: Notification)

    @Query("UPDATE notifications SET isRead = 1, readAt = :readAt WHERE id = :id")
    suspend fun markAsRead(id: Long, readAt: String)

    @Query("UPDATE notifications SET isRead = 1, readAt = :readAt WHERE userId = :userId AND isRead = 0")
    suspend fun markAllAsRead(userId: Int, readAt: String): Int

    @Query("DELETE FROM notifications WHERE id = :id")
    suspend fun deleteNotification(id: Long)

    @Query("DELETE FROM notifications WHERE userId = :userId AND (:olderThanDays IS NULL OR datetime(createdAt) < datetime('now', '-' || :olderThanDays || ' days'))")
    suspend fun clearNotifications(userId: Int, olderThanDays: Int?): Int

    @Query("DELETE FROM notifications WHERE userId = :userId")
    suspend fun clearAllNotifications(userId: Int): Int

    @Query("DELETE FROM notifications WHERE expiresAt IS NOT NULL AND datetime(expiresAt) < datetime('now')")
    suspend fun deleteExpiredNotifications(): Int
}

@Database(
    entities = [Task::class, DeltaChange::class, SyncMeta::class, Conflict::class, Notification::class],
    version = 2,
    exportSchema = false
)
abstract class AppDatabase : RoomDatabase() {
    abstract fun taskDao(): TaskDao
    abstract fun deltaQueueDao(): DeltaQueueDao
    abstract fun syncMetaDao(): SyncMetaDao
    abstract fun conflictDao(): ConflictDao
    abstract fun notificationDao(): NotificationDao

    companion object {
        @Volatile
        private var INSTANCE: AppDatabase? = null

        fun getInstance(context: Context): AppDatabase {
            return INSTANCE ?: synchronized(this) {
                INSTANCE ?: Room.databaseBuilder(
                    context.applicationContext,
                    AppDatabase::class.java,
                    "todoapp.db"
                ).build().also { INSTANCE = it }
            }
        }
    }
}
