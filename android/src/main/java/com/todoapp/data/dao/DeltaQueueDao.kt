package com.todoapp.data.dao

import androidx.room.*
import com.todoapp.data.entities.DeltaQueueEntity

@Dao
interface DeltaQueueDao {
    
    @Query("SELECT * FROM delta_queue WHERE status = 'pending' ORDER BY created_at ASC")
    suspend fun getPendingDeltas(): List<DeltaQueueEntity>
    
    @Query("SELECT * FROM delta_queue WHERE entity_type = :entityType AND status = 'pending' ORDER BY created_at ASC")
    suspend fun getPendingDeltasByEntityType(entityType: String): List<DeltaQueueEntity>
    
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertDelta(delta: DeltaQueueEntity): Long
    
    @Update
    suspend fun updateDelta(delta: DeltaQueueEntity)
    
    @Query("UPDATE delta_queue SET status = :status, retry_count = retry_count + 1 WHERE id = :id")
    suspend fun updateDeltaStatus(id: Long, status: String)
    
    @Query("DELETE FROM delta_queue WHERE status = 'synced'")
    suspend fun clearSyncedDeltas()
    
    @Query("DELETE FROM delta_queue WHERE status = 'failed' AND retry_count >= 3")
    suspend fun clearFailedDeltas()
    
    @Query("SELECT COUNT(*) FROM delta_queue WHERE status = 'pending'")
    suspend fun getPendingDeltaCount(): Int
    
    @Query("SELECT * FROM delta_queue WHERE local_id = :localId AND entity_type = :entityType LIMIT 1")
    suspend fun getDeltaByLocalId(localId: String, entityType: String): DeltaQueueEntity?
}