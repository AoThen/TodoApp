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
                parseUserIdFromToken(token)
            } else {
                "unknown-user"
            }
        } catch (e: Exception) {
            "unknown-user"
        }
    }

    private fun parseUserIdFromToken(token: String): String {
        return try {
            val parts = token.split(".")
            if (parts.size == 3) {
                val payload = android.util.Base64.decode(parts[1], android.util.Base64.DEFAULT)
                    .toString(Charsets.UTF_8)
                val json = org.json.JSONObject(payload)
                json.optString("user_id", json.optString("sub", "unknown-user"))
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

    suspend fun insertTask(task: Task): Result<Long> {
        return Result.runCatching {
            taskDao.insertTask(task)
        }
    }

    suspend fun updateTask(task: Task): Result<Unit> {
        return Result.runCatching {
            val updatedTask = task.copy(updatedAt = DateTimeUtils.getCurrentTimestamp())
            taskDao.updateTask(updatedTask)
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
            val isCompleting = task.status != "completed"
            val updatedTask = task.copy(
                status = if (isCompleting) "completed" else "todo",
                updatedAt = DateTimeUtils.getCurrentTimestamp(),
                lastModified = DateTimeUtils.getCurrentTimestamp(),
                completedAt = if (isCompleting) DateTimeUtils.getCurrentTimestamp() else null
            )
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
            clientVersion = (task.serverVersion ?: 0) + 1,
            timestamp = DateTimeUtils.getCurrentTimestamp()
        )
        deltaQueueDao.insertDelta(delta)
    }
}
