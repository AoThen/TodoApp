package com.todoapp.di

import android.content.Context
import com.todoapp.data.local.AppDatabase
import com.todoapp.data.local.ConflictDao
import com.todoapp.data.local.DeltaQueueDao
import com.todoapp.data.local.NotificationDao
import com.todoapp.data.local.SyncMetaDao
import com.todoapp.data.local.TaskDao
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
        return AppDatabase.getInstance(context)
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
    fun provideConflictDao(database: AppDatabase): ConflictDao {
        return database.conflictDao()
    }

    @Provides
    @Singleton
    fun provideNotificationDao(database: AppDatabase): NotificationDao {
        return database.notificationDao()
    }
}
