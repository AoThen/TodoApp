package com.todoapp.data.entities

import androidx.room.ColumnInfo
import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "sync_meta")
data class SyncMetaEntity(
    @PrimaryKey
    val key: String,
    
    @ColumnInfo(name = "value")
    val value: String,
    
    @ColumnInfo(name = "updated_at")
    val updatedAt: String
)