package com.todoapp.data.repository

import android.content.Context
import com.todoapp.config.AppConfig
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

    /** 任务流：按当前登录用户过滤，避免多账号在同设备上互相可见 */
    fun getAllTasks(): Flow<List<Task>> = taskDao.getAllTasks(getCurrentUserId())

    private fun getCurrentUserId(): String {
        return try {
            val token = RetrofitClient.getAccessToken(context)
            if (token.isNotEmpty()) {
                JwtUtils.parseUserId(token)
            } else {
                AppConfig.GUEST_USER_ID
            }
        } catch (e: Exception) {
            AppConfig.GUEST_USER_ID
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

    /** 按截止日期区间查询任务（日历视图使用），限定当前用户防止多账号串号 */
    suspend fun getTasksDueBetween(startIso: String, endIso: String): Result<List<Task>> {
        return Result.runCatching {
            taskDao.getTasksDueBetween(getCurrentUserId(), startIso, endIso)
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

    /**
     * 手动排序：更新任务 sort_order 并入 delta 队列。
     * payload 为 Task 全量快照（camelCase），由 SyncPayloadConverter 翻译为服务端 snake_case 字段。
     */
    suspend fun persistSortOrder(task: Task, newSortOrder: Double): Result<Unit> {
        return Result.runCatching {
            val updatedTask = task.copy(
                sortOrder = newSortOrder,
                updatedAt = DateTimeUtils.getCurrentTimestamp(),
                lastModified = DateTimeUtils.getCurrentTimestamp()
            )
            taskDao.updateTask(updatedTask)
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

    /** 登出时清空待同步队列：避免前一账号的离线修改以新账号身份上传，造成跨账号数据串写 */
    suspend fun clearDeltaQueue(): Result<Unit> {
        return Result.runCatching {
            deltaQueueDao.deleteAllDeltas()
        }
    }

    /**
     * 游客任务并入登录账号：仅迁移任务归属，待同步队列原样保留，
     * 后续同步会以新账号身份自动上传，实现"离线记账、登录后合并"。
     * @return 迁移的任务条数
     */
    suspend fun migrateGuestTasks(newUserId: String): Result<Int> {
        return Result.runCatching {
            taskDao.migrateTasksUserId(AppConfig.GUEST_USER_ID, newUserId)
        }
    }

    suspend fun getSyncMeta(userId: String): Result<SyncMeta?> {
        return Result.runCatching {
            syncMetaDao.getSyncMeta(userId)
        }
    }

    suspend fun updateSyncMeta(userId: String, lastSyncAt: String): Result<Unit> {
        return Result.runCatching {
            // 保留已有的 lastServerVersion，避免仅更新时间戳时把版本号重置为 0
            val previous = syncMetaDao.getSyncMeta(userId)
            syncMetaDao.insertSyncMeta(
                SyncMeta(
                    userId = userId,
                    lastSyncAt = lastSyncAt,
                    lastServerVersion = previous?.lastServerVersion ?: 0
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
        if (operation == "insert") {
            deltaQueueDao.insertDelta(delta)
            return
        }
        // 合并同一任务的重复修改：payload 为任务全量快照，新 delta 完全覆盖旧 mutation，
        // 避免陈旧 delta 反复重发触发服务端冲突合并
        if (task.serverId != null) {
            // 已同步任务：直接替换其全部待同步 delta
            deltaQueueDao.deleteDeltasByLocalId(task.localId)
        } else {
            // 未同步任务：insert 必须保留（服务端尚未见过该任务），仅合并旧的 update/delete
            deltaQueueDao.deleteMutationsByLocalId(task.localId)
        }
        deltaQueueDao.insertDelta(delta)
    }
}
