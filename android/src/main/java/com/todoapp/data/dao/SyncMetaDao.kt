package com.todoapp.data.dao

import androidx.room.*
import com.todoapp.data.entities.SyncMetaEntity

@Dao
interface SyncMetaDao {
    
    @Query("SELECT * FROM sync_meta WHERE key = :key LIMIT 1")
    suspend fun getMetaByKey(key: String): SyncMetaEntity?
    
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertMeta(meta: SyncMetaEntity)
    
    @Update
    suspend fun updateMeta(meta: SyncMetaEntity)
    
    @Query("DELETE FROM sync_meta WHERE key = :key")
    suspend fun deleteMeta(key: String)
    
    @Query("SELECT value FROM sync_meta WHERE key = :key LIMIT 1")
    suspend fun getValueByKey(key: String): String?
    
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun setKeyValue(key: String, value: String, updatedAt: String) {
        insertMeta(SyncMetaEntity(key, value, updatedAt))
    }
}