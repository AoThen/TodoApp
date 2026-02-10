package com.todoapp.data.entities

import androidx.room.ColumnInfo
import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "delta_queue")
data class DeltaQueueEntity(
    @PrimaryKey(autoGenerate = true)
    val id: Long = 0,
    
    @ColumnInfo(name = "local_id")
    val localId: String,
    
    @ColumnInfo(name = "operation")
    val operation: String, // create, update, delete
    
    @ColumnInfo(name = "payload")
    val payload: String, // JSON string
    
    @ColumnInfo(name = "entity_type")
    val entityType: String, // task, notification
    
    @ColumnInfo(name = "created_at")
    val createdAt: String,
    
    @ColumnInfo(name = "retry_count")
    val retryCount: Int = 0,
    
    @ColumnInfo(name = "status")
    val status: String = "pending" // pending, syncing, synced, failed
)