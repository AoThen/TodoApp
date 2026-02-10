package com.todoapp.di

import android.content.Context
import androidx.room.Room
import com.todoapp.data.database.AppDatabase
import com.todoapp.data.dao.DeltaQueueDao
import com.todoapp.data.dao.NotificationDao
import com.todoapp.data.dao.SyncMetaDao
import com.todoapp.data.dao.TaskDao
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object DatabaseModule {

    @Provides
    @Singleton
    fun provideDatabase(@ApplicationContext context: Context): AppDatabase {
        return Room.databaseBuilder(
            context.applicationContext,
            AppDatabase::class.java,
            "todoapp_database"
        )
        .fallbackToDestructiveMigration()
        .build()
    }

    @Provides
    @Singleton
    fun provideTaskDao(database: AppDatabase): TaskDao {
        return database.taskDao()
    }

    @Provides
    @Singleton
    fun provideDeltaQueueDao(database: AppDatabase): DeltaQueueDao {
        return database.deltaQueueDao()
    }

    @Provides
    @Singleton
    fun provideSyncMetaDao(database: AppDatabase): SyncMetaDao {
        return database.syncMetaDao()
    }

    

    @Provides
    @Singleton
    fun provideNotificationDao(database: AppDatabase): NotificationDao {
        return database.notificationDao()
    }
}
