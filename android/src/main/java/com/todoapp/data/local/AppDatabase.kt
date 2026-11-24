package com.todoapp.data.local

import android.content.Context
import androidx.annotation.VisibleForTesting
import androidx.room.*
import androidx.room.migration.Migration
import androidx.sqlite.db.SupportSQLiteDatabase
import kotlinx.coroutines.flow.Flow

@Entity(tableName = "tasks", indices = [
    Index(value = ["userId"]),
    Index(value = ["status"]),
    Index(value = ["localId"]),
    Index(value = ["serverId"])
])
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
    // 勾选完成前的状态：取消完成时恢复，避免 in_progress/archived 状态被抹掉（仅本地使用，不参与同步）
    val previousStatus: String? = null,
    val priority: String = "medium",
    val dueAt: String? = null,
    val createdAt: String,
    val updatedAt: String,
    val completedAt: String? = null,
    val isDeleted: Boolean = false,
    val isArchived: Boolean = false,
    val archivedAt: String? = null,
    val lastModified: String,
    // 手动排序值：越小越靠前；缺省 0（manual 模式下同值回退创建时间倒序）
    val sortOrder: Double = 0.0,
    // 循环规则："" / daily / weekly / monthly / weekdays（与服务端约定）
    val recurrenceType: String = "",
    // 循环系列 ID：同一系列任务共享，服务端据此生成下一次 occurrence
    val recurrenceSeriesId: String = ""
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
    @Query("SELECT * FROM tasks WHERE userId = :userId AND isDeleted = 0 ORDER BY createdAt DESC")
    fun getAllTasks(userId: String): Flow<List<Task>>

    @Query("SELECT * FROM tasks WHERE localId = :localId")
    suspend fun getTaskByLocalId(localId: String): Task?

    @Query("SELECT * FROM tasks WHERE id = :id")
    suspend fun getTaskById(id: Long): Task?

    /** 按服务端 ID 查询当前用户任务：不同账号的 serverId 可能重复，必须带 userId 过滤防止串号 */
    @Query("SELECT * FROM tasks WHERE userId = :userId AND serverId = :serverId LIMIT 1")
    suspend fun getTaskByServerId(userId: String, serverId: Long): Task?

    /** 按本地 ID 查找尚未关联服务端 ID 的未删除任务（同步时用于精确合并离线创建的任务，避免重复） */
    @Query("SELECT * FROM tasks WHERE isDeleted = 0 AND serverId IS NULL AND localId = :localId LIMIT 1")
    suspend fun getUnsyncedTaskByLocalId(localId: String): Task?

    /** 查询截止日期落在 [startIso, endIso] 区间内的未删除任务（dueAt 为 ISO-8601 UTC 字符串，可直接做字典序比较） */
    @Query("SELECT * FROM tasks WHERE userId = :userId AND isDeleted = 0 AND dueAt IS NOT NULL AND dueAt >= :startIso AND dueAt <= :endIso ORDER BY dueAt ASC")
    suspend fun getTasksDueBetween(userId: String, startIso: String, endIso: String): List<Task>

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
    // 追加 id ASC：时间戳为秒级精度，同秒入队的 delta 依插入顺序稳定排序
    @Query("SELECT * FROM delta_queue ORDER BY timestamp ASC, id ASC")
    suspend fun getAllDeltas(): List<DeltaChange>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertDelta(delta: DeltaChange): Long

    @Query("DELETE FROM delta_queue WHERE id = :id")
    suspend fun deleteDelta(id: Long)

    @Query("DELETE FROM delta_queue WHERE localId = :localId")
    suspend fun deleteDeltasByLocalId(localId: String)

    @Query("DELETE FROM delta_queue WHERE localId = :localId AND op = :op")
    suspend fun deleteDeltasByLocalIdAndOp(localId: String, op: String)

    /** 删除同一任务的待同步 update/delete（insert 需保留，服务端尚未见过该任务） */
    @Query("DELETE FROM delta_queue WHERE localId = :localId AND op IN ('update', 'delete')")
    suspend fun deleteMutationsByLocalId(localId: String)

    /** 统计同一任务仍待同步的 update/delete 数量：大于 0 说明本地有未确认的新修改 */
    @Query("SELECT COUNT(*) FROM delta_queue WHERE localId = :localId AND op IN ('update', 'delete')")
    suspend fun countPendingMutations(localId: String): Int

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
    version = 5,
    exportSchema = true
)
abstract class AppDatabase : RoomDatabase() {
    abstract fun taskDao(): TaskDao
    abstract fun deltaQueueDao(): DeltaQueueDao
    abstract fun syncMetaDao(): SyncMetaDao
    abstract fun conflictDao(): ConflictDao
    abstract fun notificationDao(): NotificationDao

    companion object {
        private const val DATABASE_NAME = "todoapp.db"
        private const val VERSION_1 = 1
        private const val VERSION_2 = 2
        private const val VERSION_3 = 3
        private const val VERSION_4 = 4
        private const val VERSION_5 = 5

        @Volatile
        private var INSTANCE: AppDatabase? = null

        private val MIGRATION_1_2 = object : Migration(VERSION_1, VERSION_2) {
            override fun migrate(db: SupportSQLiteDatabase) {
                // Version 1 to 2: Added indexes to notifications table for better query performance
                db.execSQL("CREATE INDEX IF NOT EXISTS index_notifications_userId ON notifications(userId)")
                db.execSQL("CREATE INDEX IF NOT EXISTS index_notifications_isRead ON notifications(isRead)")
                db.execSQL("CREATE INDEX IF NOT EXISTS index_notifications_createdAt ON notifications(createdAt)")
            }
        }

        private val MIGRATION_2_3 = object : Migration(VERSION_2, VERSION_3) {
            override fun migrate(db: SupportSQLiteDatabase) {
                // Version 2 to 3: Added isArchived field to tasks for better task management
                db.execSQL("ALTER TABLE tasks ADD COLUMN isArchived INTEGER NOT NULL DEFAULT 0")
                db.execSQL("ALTER TABLE tasks ADD COLUMN archivedAt TEXT DEFAULT NULL")
                db.execSQL("CREATE INDEX IF NOT EXISTS index_tasks_userId ON tasks(userId)")
                db.execSQL("CREATE INDEX IF NOT EXISTS index_tasks_status ON tasks(status)")
            }
        }

        private val MIGRATION_3_4 = object : Migration(VERSION_3, VERSION_4) {
            override fun migrate(db: SupportSQLiteDatabase) {
                // Version 3 to 4: tasks 增加勾选前状态列；为 localId/serverId 查询建索引
                db.execSQL("ALTER TABLE tasks ADD COLUMN previousStatus TEXT DEFAULT NULL")
                db.execSQL("CREATE INDEX IF NOT EXISTS index_tasks_localId ON tasks(localId)")
                db.execSQL("CREATE INDEX IF NOT EXISTS index_tasks_serverId ON tasks(serverId)")
            }
        }

        private val MIGRATION_4_5 = object : Migration(VERSION_4, VERSION_5) {
            override fun migrate(db: SupportSQLiteDatabase) {
                // Version 4 to 5: tasks 增加手动排序与循环任务列
                db.execSQL("ALTER TABLE tasks ADD COLUMN sortOrder REAL NOT NULL DEFAULT 0")
                db.execSQL("ALTER TABLE tasks ADD COLUMN recurrenceType TEXT NOT NULL DEFAULT ''")
                db.execSQL("ALTER TABLE tasks ADD COLUMN recurrenceSeriesId TEXT NOT NULL DEFAULT ''")
            }
        }

        fun getInstance(context: Context): AppDatabase {
            return INSTANCE ?: synchronized(this) {
                INSTANCE ?: Room.databaseBuilder(
                    context.applicationContext,
                    AppDatabase::class.java,
                    DATABASE_NAME
                )
                    .addMigrations(MIGRATION_1_2, MIGRATION_2_3, MIGRATION_3_4, MIGRATION_4_5)
                    .build()
                    .also { INSTANCE = it }
            }
        }

        @VisibleForTesting
        fun getInMemoryInstance(context: Context): AppDatabase {
            return Room.inMemoryDatabaseBuilder(
                context.applicationContext,
                AppDatabase::class.java
            )
                .addMigrations(MIGRATION_1_2, MIGRATION_2_3, MIGRATION_3_4, MIGRATION_4_5)
                .build()
        }
    }
}
