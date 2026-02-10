package com.todoapp.data.entities

import androidx.room.ColumnInfo
import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "notifications")
data class NotificationEntity(
    @PrimaryKey(autoGenerate = true)
    val id: Long = 0,
    
    @ColumnInfo(name = "server_id")
    val serverId: Long?,
    
    @ColumnInfo(name = "title")
    val title: String,
    
    @ColumnInfo(name = "message")
    val message: String,
    
    @ColumnInfo(name = "priority")
    val priority: String, // urgent, high, normal, low
    
    @ColumnInfo(name = "is_read")
    val isRead: Boolean = false,
    
    @ColumnInfo(name = "created_at")
    val createdAt: String,
    
    @ColumnInfo(name = "read_at")
    val readAt: String?,
    
    @ColumnInfo(name = "action_url")
    val actionUrl: String?,
    
    @ColumnInfo(name = "action_type")
    val actionType: String?
)