package com.todoapp.data.repository

import android.content.Context
import com.todoapp.data.local.DeltaChange
import com.todoapp.data.local.DeltaQueueDao
import com.todoapp.data.local.SyncMeta
import com.todoapp.data.local.SyncMetaDao
import com.todoapp.data.local.Task
import com.todoapp.data.local.TaskDao
import com.todoapp.data.remote.RetrofitClient
import com.todoapp.utils.DateTimeUtils
import com.todoapp.utils.JwtUtils
import com.todoapp.utils.Result
import com.google.gson.Gson
import kotlinx.coroutines.flow.Flow
import java.util.UUID

class TaskRepository(
    private val taskDao: TaskDao,
    private val deltaQueueDao: DeltaQueueDao,
    private val syncMetaDao: SyncMetaDao,
    private val context: Context
) {
    val allTasks: Flow<List<Task>> = taskDao.getAllTasks()

    private fun getCurrentUserId(): String {
        return try {
            val token = RetrofitClient.getAccessToken(context)
            if (token.isNotEmpty()) {
                JwtUtils.parseUserId(token)
            } else {
                "unknown-user"
            }
        } catch (e: Exception) {
            "unknown-user"
        }
    }

    suspend fun getTaskById(id: Long): Result<Task> {
        return Result.runCatching {
            taskDao.getTaskById(id) ?: throw NoSuchElementException("Task not found with id: $id")
        }
    }

    suspend fun getTaskByLocalId(localId: String): Result<Task> {
        return Result.runCatching {
            taskDao.getTaskByLocalId(localId) ?: throw NoSuchElementException("Task not found with localId: $localId")
        }
    }

    /** 按截止日期区间查询任务（日历视图使用） */
    suspend fun getTasksDueBetween(startIso: String, endIso: String): Result<List<Task>> {
        return Result.runCatching {
            taskDao.getTasksDueBetween(startIso, endIso)
        }
    }

    suspend fun insertTask(task: Task): Result<Long> {
        return Result.runCatching {
            taskDao.insertTask(task)
        }
    }

    suspend fun updateTask(task: Task): Result<Unit> {
        return Result.runCatching {
            val updatedTask = task.copy(
                updatedAt = DateTimeUtils.getCurrentTimestamp(),
                lastModified = DateTimeUtils.getCurrentTimestamp()
            )
            taskDao.updateTask(updatedTask)

            // 编辑同样进入同步队列，避免服务器变更下发时用旧值覆盖本地编辑
            addToDeltaQueue("update", updatedTask)
        }
    }

    suspend fun deleteTask(task: Task): Result<Unit> {
        return Result.runCatching {
            val deletedTask = task.copy(
                isDeleted = true,
                updatedAt = DateTimeUtils.getCurrentTimestamp(),
                lastModified = DateTimeUtils.getCurrentTimestamp()
            )
            taskDao.updateTask(deletedTask)

            addToDeltaQueue("delete", task)
        }
    }

    suspend fun toggleTaskCompletion(task: Task): Result<Unit> {
        return Result.runCatching {
            val now = DateTimeUtils.getCurrentTimestamp()
            val updatedTask = if (task.status != "done") {
                // 完成：记住勾选前状态，取消完成时恢复，避免 in_progress/archived 被抹掉
                task.copy(
                    status = "done",
                    previousStatus = task.status,
                    updatedAt = now,
                    lastModified = now,
                    completedAt = now
                )
            } else {
                task.copy(
                    status = task.previousStatus ?: "todo",
                    previousStatus = null,
                    updatedAt = now,
                    lastModified = now,
                    completedAt = null
                )
            }
            taskDao.updateTask(updatedTask)

            addToDeltaQueue("update", updatedTask)
        }
    }

    suspend fun createTask(
        title: String,
        description: String = "",
        status: String = "todo",
        priority: String = "medium",
        dueAt: String? = null
    ): Result<Task> {
        return Result.runCatching {
            val timestamp = DateTimeUtils.getCurrentTimestamp()
            val localId = UUID.randomUUID().toString()
            val userId = getCurrentUserId()

            val task = Task(
                localId = localId,
                userId = userId,
                title = title,
                description = description,
                status = status,
                priority = priority,
                dueAt = dueAt,
                createdAt = timestamp,
                updatedAt = timestamp,
                lastModified = timestamp
            )

            val taskId = taskDao.insertTask(task)

            val delta = DeltaChange(
                localId = localId,
                op = "insert",
                payload = Gson().toJson(task),
                clientVersion = 1,
                timestamp = timestamp
            )
            deltaQueueDao.insertDelta(delta)

            task.copy(id = taskId)
        }
    }

    suspend fun getPendingDeltas(): Result<List<DeltaChange>> {
        return Result.runCatching {
            deltaQueueDao.getAllDeltas()
        }
    }

    suspend fun clearDelta(id: Long): Result<Unit> {
        return Result.runCatching {
            deltaQueueDao.deleteDelta(id)
        }
    }

    suspend fun getSyncMeta(userId: String): Result<SyncMeta?> {
        return Result.runCatching {
            syncMetaDao.getSyncMeta(userId)
        }
    }

    suspend fun updateSyncMeta(userId: String, lastSyncAt: String): Result<Unit> {
        return Result.runCatching {
            syncMetaDao.insertSyncMeta(
                SyncMeta(
                    userId = userId,
                    lastSyncAt = lastSyncAt
                )
            )
        }
    }

    private suspend fun addToDeltaQueue(operation: String, task: Task) {
        val delta = DeltaChange(
            localId = task.localId,
            op = operation,
            payload = Gson().toJson(task),
            clientVersion = task.serverVersion + 1,
            timestamp = DateTimeUtils.getCurrentTimestamp()
        )
        deltaQueueDao.insertDelta(delta)
    }
}
