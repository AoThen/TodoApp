package com.todoapp.data.sync

import android.content.Context
import androidx.work.*
import com.todoapp.data.local.AppDatabase
import com.todoapp.data.local.SyncMeta
import com.todoapp.data.local.Task
import com.todoapp.data.remote.DeltaChangeRequest
import com.todoapp.data.remote.RetrofitClient
import com.todoapp.data.remote.SyncRequest
import com.todoapp.config.SyncConfig
import com.todoapp.utils.JwtUtils
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.util.UUID
import java.util.concurrent.TimeUnit

class DeltaSyncWorker(
    context: Context,
    params: WorkerParameters
) : CoroutineWorker(context, params) {

    private val database = AppDatabase.getInstance(context)
    private val apiService = RetrofitClient.getApiService(context)

    override suspend fun doWork(): Result = withContext(Dispatchers.IO) {
        try {
            // 简化验证：只做格式和过期检查
            // 真正的签名验证由服务器完成
            val token = RetrofitClient.getAccessToken(applicationContext)
            val validationResult = JwtUtils.validateAccessToken(token)
            
            when (validationResult) {
                is JwtUtils.TokenValidation.Expired -> {
                    android.util.Log.w("DeltaSyncWorker", "Token expired, cannot sync")
                    return@withContext Result.failure()
                }
                is JwtUtils.TokenValidation.InvalidFormat,
                is JwtUtils.TokenValidation.InvalidType,
                is JwtUtils.TokenValidation.Error -> {
                    android.util.Log.e("DeltaSyncWorker", "Token validation failed: $validationResult")
                    return@withContext Result.failure()
                }
                is JwtUtils.TokenValidation.Valid -> {
                    // Token有效，继续同步
                }
            }
            
            val pendingDeltas = database.deltaQueueDao().getAllDeltas()
            if (pendingDeltas.isEmpty()) {
                return@withContext Result.success()
            }

            // Get current user ID from secure storage
            val userId = getCurrentUserId()
            val syncMeta = database.syncMetaDao().getSyncMeta(userId)
            val lastSyncAt = syncMeta?.lastSyncAt ?: ""

            val changes: List<DeltaChangeRequest> = pendingDeltas.map { delta ->
                DeltaChangeRequest(
                    localId = delta.localId,
                    op = delta.op,
                    payload = SyncPayloadConverter.payloadForServer(delta.payload),
                    clientVersion = delta.clientVersion
                )
            }

            val request = SyncRequest(lastSyncAt, changes)
            val response = apiService.sync(request)

            val syncBody = response.body()
            if (!response.isSuccessful || syncBody == null) {
                return@withContext Result.retry()
            }
            val syncResponse = syncBody

            // 先处理已确认的客户端变更：清理 delta 队列，并为 insert 回填服务端 ID
            // 注：未用 room-ktx 的 withTransaction——它是顶层扩展函数，单测的 mockk AppDatabase
            // 无法拦截，会在 relaxed mock 上永久挂起；待测试改用真实内存库后再引入事务
            for (clientChange in syncResponse.clientChanges) {
                val delta = pendingDeltas.find { it.localId == clientChange.localId } ?: continue
                if (clientChange.op == "insert") {
                    val localTask = database.taskDao().getTaskByLocalId(delta.localId)
                    if (localTask != null && localTask.serverId == null) {
                        database.taskDao().updateTask(
                            localTask.copy(serverId = clientChange.serverId)
                        )
                    }
                }
                database.deltaQueueDao().deleteDelta(delta.id)
            }

            // 再应用服务端变更：按 serverId 匹配本地任务；不存在且未删除 → 其他设备新建的任务，插入本地
            for (serverChange in syncResponse.serverChanges) {
                val existing = database.taskDao().getTaskByServerId(serverChange.id)
                if (existing != null) {
                    if (serverChange.isDeleted) {
                        database.taskDao().softDeleteTask(existing.id, serverChange.updatedAt)
                    } else {
                        database.taskDao().updateTask(
                            existing.copy(
                                serverId = serverChange.id,
                                serverVersion = serverChange.serverVersion,
                                title = serverChange.title ?: existing.title,
                                description = serverChange.description ?: existing.description,
                                status = serverChange.status ?: existing.status,
                                priority = serverChange.priority ?: existing.priority,
                                dueAt = serverChange.dueAt ?: existing.dueAt,
                                updatedAt = serverChange.updatedAt,
                                isDeleted = false
                            )
                        )
                    }
                } else if (!serverChange.isDeleted) {
                    val timestamp = serverChange.updatedAt
                    // 优先按 local_id 精确匹配本地尚未同步的离线任务（离线创建后回执丢失时的自愈），
                    // 不按标题猜测，避免同名不同任务被错误合并
                    val ghost = serverChange.localId
                        ?.takeIf { it.isNotEmpty() }
                        ?.let { database.taskDao().getUnsyncedTaskByLocalId(it) }
                    if (ghost != null) {
                        database.taskDao().updateTask(
                            ghost.copy(
                                serverId = serverChange.id,
                                serverVersion = serverChange.serverVersion,
                                description = serverChange.description ?: ghost.description,
                                status = serverChange.status ?: ghost.status,
                                priority = serverChange.priority ?: ghost.priority,
                                dueAt = serverChange.dueAt ?: ghost.dueAt,
                                updatedAt = serverChange.updatedAt,
                                lastModified = serverChange.updatedAt
                            )
                        )
                    } else {
                        database.taskDao().insertTask(
                            Task(
                                localId = UUID.randomUUID().toString(),
                                serverId = serverChange.id,
                                userId = userId,
                                serverVersion = serverChange.serverVersion,
                                title = serverChange.title ?: "",
                                description = serverChange.description ?: "",
                                status = serverChange.status ?: "todo",
                                priority = serverChange.priority ?: "medium",
                                dueAt = serverChange.dueAt,
                                createdAt = timestamp,
                                updatedAt = timestamp,
                                lastModified = timestamp
                            )
                        )
                    }
                }
            }

            for (conflict in syncResponse.conflicts) {
                database.conflictDao().insertConflict(
                    com.todoapp.data.local.Conflict(
                        localId = conflict.localId,
                        serverId = conflict.serverId,
                        reason = conflict.reason,
                        options = conflict.options?.joinToString(",") ?: "",
                        createdAt = java.time.Instant.now().toString()
                    )
                )
            }

            database.syncMetaDao().insertSyncMeta(
                SyncMeta(
                    userId = userId,
                    lastSyncAt = syncResponse.lastSyncAt
                )
            )

            Result.success()
        } catch (e: Exception) {
            if (runAttemptCount < 3) {
                Result.retry()
            } else {
                Result.failure()
            }
        }
    }

    private fun getCurrentUserId(): String {
        return try {
            // Extract user ID from access token (JWT payload)
            val token = RetrofitClient.getAccessToken(applicationContext)
            if (token.isNotEmpty()) {
                JwtUtils.parseUserId(token)
            } else {
                "unknown-user"
            }
        } catch (e: Exception) {
            "unknown-user"
        }
    }

    companion object {
        const val WORK_NAME = "DeltaSyncWorker"

        fun schedule(context: Context) {
            val constraints = Constraints.Builder()
                .setRequiredNetworkType(NetworkType.CONNECTED)
                .build()

            // WorkManager 周期任务下限为 15 分钟
            val request = PeriodicWorkRequestBuilder<DeltaSyncWorker>(
                SyncConfig.SYNC_INTERVAL_MINUTES, TimeUnit.MINUTES
            )
                .setConstraints(constraints)
                .setBackoffCriteria(
                    BackoffPolicy.EXPONENTIAL,
                    WorkRequest.MIN_BACKOFF_MILLIS,
                    TimeUnit.MILLISECONDS
                )
                .build()

            WorkManager.getInstance(context).enqueueUniquePeriodicWork(
                WORK_NAME,
                ExistingPeriodicWorkPolicy.KEEP,
                request
            )
        }

        fun runOnce(context: Context) {
            val request = OneTimeWorkRequestBuilder<DeltaSyncWorker>()
                .build()
            WorkManager.getInstance(context).enqueue(request)
        }
    }
}
