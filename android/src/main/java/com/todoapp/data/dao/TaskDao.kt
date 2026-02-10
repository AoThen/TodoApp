package com.todoapp.data.dao

import androidx.room.*
import com.todoapp.data.entities.TaskEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface TaskDao {
    
    @Query("SELECT * FROM tasks WHERE is_deleted = 0 ORDER BY created_at DESC")
    fun getAllTasks(): Flow<List<TaskEntity>>
    
    @Query("SELECT * FROM tasks WHERE id = :id AND is_deleted = 0 LIMIT 1")
    suspend fun getTaskById(id: Long): TaskEntity?
    
    @Query("SELECT * FROM tasks WHERE server_id = :serverId AND is_deleted = 0 LIMIT 1")
    suspend fun getTaskByServerId(serverId: Long): TaskEntity?
    
    @Query("SELECT * FROM tasks WHERE status = :status AND is_deleted = 0 ORDER BY created_at DESC")
    fun getTasksByStatus(status: String): Flow<List<TaskEntity>>
    
    @Query("SELECT * FROM tasks WHERE priority = :priority AND is_deleted = 0 ORDER BY created_at DESC")
    fun getTasksByPriority(priority: String): Flow<List<TaskEntity>>
    
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertTask(task: TaskEntity): Long
    
    @Update
    suspend fun updateTask(task: TaskEntity)
    
    @Query("UPDATE tasks SET is_deleted = 1, updated_at = :updatedAt WHERE id = :id")
    suspend fun deleteTask(id: Long, updatedAt: String)
    
    @Query("DELETE FROM tasks WHERE is_deleted = 1")
    suspend fun purgeDeletedTasks()
    
    @Query("SELECT * FROM tasks WHERE sync_status = 'pending' AND is_deleted = 0")
    suspend fun getPendingSyncTasks(): List<TaskEntity>
    
    @Query("UPDATE tasks SET sync_status = :status WHERE id = :id")
    suspend fun updateTaskSyncStatus(id: Long, status: String)
    
    @Query("SELECT COUNT(*) FROM tasks WHERE is_deleted = 0")
    suspend fun getTaskCount(): Int
    
    @Query("SELECT * FROM tasks WHERE title LIKE '%' || :query || '%' AND is_deleted = 0 ORDER BY created_at DESC")
    suspend fun searchTasks(query: String): List<TaskEntity>
}