package com.todoapp.data.database

import android.content.Context
import androidx.room.*
import androidx.sqlite.db.SupportSQLiteDatabase
import com.todoapp.data.dao.*
import com.todoapp.data.entities.*
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch

@Database(
    entities = [
        TaskEntity::class,
        DeltaQueueEntity::class,
        NotificationEntity::class,
        SyncMetaEntity::class
    ],
    version = 3,
    exportSchema = false
)
@TypeConverters(Converters::class)
abstract class AppDatabase : RoomDatabase() {
    
    abstract fun taskDao(): TaskDao
    abstract fun deltaQueueDao(): DeltaQueueDao
    abstract fun notificationDao(): NotificationDao
    abstract fun syncMetaDao(): SyncMetaDao
    
    companion object {
        @Volatile
        private var INSTANCE: AppDatabase? = null
        
        private const val DATABASE_NAME = "todoapp_database"
        
        fun getDatabase(context: Context): AppDatabase {
            return INSTANCE ?: synchronized(this) {
                val instance = Room.databaseBuilder(
                    context.applicationContext,
                    AppDatabase::class.java,
                    DATABASE_NAME
                )
                .addCallback(DatabaseCallback())
                .fallbackToDestructiveMigration()
                .build()
                INSTANCE = instance
                instance
            }
        }
        
        private class DatabaseCallback : RoomDatabase.Callback() {
            override fun onCreate(db: SupportSQLiteDatabase) {
                super.onCreate(db)
                INSTANCE?.let { database ->
                    CoroutineScope(Dispatchers.IO).launch {
                        populateInitialData(database)
                    }
                }
            }
        }
        
        private suspend fun populateInitialData(database: AppDatabase) {
            val syncMetaDao = database.syncMetaDao()
            
            // Initialize sync metadata
            val currentTime = System.currentTimeMillis().toString()
            syncMetaDao.setKeyValue("last_sync_at", "0", currentTime)
            syncMetaDao.setKeyValue("sync_version", "1", currentTime)
            syncMetaDao.setKeyValue("device_id", "default-device", currentTime)
        }
    }
}

class Converters {
    @TypeConverter
    fun fromStringList(value: List<String>): String {
        return value.joinToString(",")
    }
    
    @TypeConverter
    fun toStringList(value: String): List<String> {
        return if (value.isEmpty()) emptyList() else value.split(",")
    }
}