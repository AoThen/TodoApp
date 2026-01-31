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

@Database(
    entities = [Task::class, DeltaChange::class, SyncMeta::class, Conflict::class],
    version = 1,
    exportSchema = false
)
abstract class AppDatabase : RoomDatabase() {
    abstract fun taskDao(): TaskDao
    abstract fun deltaQueueDao(): DeltaQueueDao
    abstract fun syncMetaDao(): SyncMetaDao
    abstract fun conflictDao(): ConflictDao

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
