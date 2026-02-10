package com.todoapp.data.entities

import androidx.room.ColumnInfo
import androidx.room.Entity
import androidx.room.Index
import androidx.room.PrimaryKey

@Entity(
    tableName = "tasks",
    indices = [
        Index(value = ["status"]),
        Index(value = ["priority"]),
        Index(value = ["server_id"]),
        Index(value = ["is_deleted"])
    ]
)
data class TaskEntity(
    @PrimaryKey(autoGenerate = true)
    val id: Long = 0,
    
    @ColumnInfo(name = "server_id")
    val serverId: Long?,
    
    @ColumnInfo(name = "title")
    val title: String,
    
    @ColumnInfo(name = "description")
    val description: String?,
    
    @ColumnInfo(name = "status")
    val status: String,
    
    @ColumnInfo(name = "priority")
    val priority: String?,
    
    @ColumnInfo(name = "due_at")
    val dueAt: String?,
    
    @ColumnInfo(name = "created_at")
    val createdAt: String?,
    
    @ColumnInfo(name = "updated_at")
    val updatedAt: String?,
    
    @ColumnInfo(name = "completed_at")
    val completedAt: String?,
    
    @ColumnInfo(name = "local_version")
    val localVersion: Int = 1,
    
    @ColumnInfo(name = "server_version")
    val serverVersion: Int?,
    
    @ColumnInfo(name = "is_deleted")
    val isDeleted: Boolean = false,
    
    @ColumnInfo(name = "sync_status")
    val syncStatus: String = "pending" // pending, synced, conflict
)