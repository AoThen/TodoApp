package com.todoapp.di

import com.todoapp.data.local.DeltaQueueDao
import com.todoapp.data.local.SyncMetaDao
import com.todoapp.data.local.TaskDao
import com.todoapp.data.repository.TaskRepository
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object RepositoryModule {

    @Provides
    @Singleton
    fun provideTaskRepository(
        taskDao: TaskDao,
        deltaQueueDao: DeltaQueueDao,
        syncMetaDao: SyncMetaDao
    ): TaskRepository {
        return TaskRepository(taskDao, deltaQueueDao, syncMetaDao)
    }
}
