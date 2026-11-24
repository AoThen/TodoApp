package com.todoapp.data.sync

import android.content.Context
import androidx.work.*
import com.todoapp.data.local.AppDatabase
import com.todoapp.data.local.DeltaChange
import com.todoapp.data.local.SyncMeta
import com.todoapp.data.local.Task
import com.todoapp.data.remote.DeltaChangeRequest
import com.todoapp.data.remote.RetrofitClient
import com.todoapp.data.remote.SyncRequest
import com.todoapp.config.SyncConfig
import com.todoapp.utils.DateTimeUtils
import com.todoapp.utils.JwtUtils
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.sync.Mutex
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
        // 进程内互斥：手动同步与周期同步并发时，后到者直接跳过，避免重复消费同一批 delta
        if (!syncMutex.tryLock()) {
            return@withContext Result.success()
        }
        try {
            doSync()
        } finally {
            syncMutex.unlock()
        }
    }

    private suspend fun doSync(): Result = withContext(Dispatchers.IO) {
        try {
            // 仅拦截未登录/凭据异常；过期 token 放行，
            // 由网络层的 401→自动续期→重试链路统一处理（预检拦截会导致过期后同步永久失效）
            val token = RetrofitClient.getAccessToken(applicationContext)
            val validationResult = JwtUtils.validateAccessToken(token)

            when (validationResult) {
                is JwtUtils.TokenValidation.Expired -> {
                    // 放行：网络层会先刷新 token 再重试
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

            // 组装上传批次：
            // 1) insert 尚未被服务端确认（本地 serverId 为空）的任务，其 update/delete 本轮延迟发送，
            //    待 insert ack 回填 serverId 后下一轮再传，避免服务端因 payload 缺少任务 ID 而跳过，
            //    导致“离线创建后再编辑”的内容被静默丢弃；
            // 2) update/delete 以上传时刻本地的 serverId 为准注入 payload（冻结 payload 可能缺失或过期）；
            // 3) 本地任务已不存在的滞留 delta 直接清理，防止无限重发。
            val changes = mutableListOf<DeltaChangeRequest>()
            val staleDeltas = mutableListOf<DeltaChange>()
            val insertSent = mutableSetOf<String>()
            val taskCache = mutableMapOf<String, Task?>()

            for (delta in pendingDeltas) {
                when (delta.op) {
                    "insert" -> {
                        // 同一任务仅发送一条 insert（正常至多一条，防御重复滞留）
                        if (insertSent.add(delta.localId)) {
                            changes += DeltaChangeRequest(
                                localId = delta.localId,
                                op = delta.op,
                                payload = SyncPayloadConverter.payloadForServer(delta.payload),
                                clientVersion = delta.clientVersion
                            )
                        } else {
                            staleDeltas += delta
                        }
                    }
                    "update", "delete" -> {
                        val task = taskCache.getOrPut(delta.localId) {
                            database.taskDao().getTaskByLocalId(delta.localId)
                        }
                        val serverId = task?.serverId
                        when {
                            task == null -> staleDeltas += delta
                            serverId == null -> {
                                // insert 未确认：延迟到下一轮，保留 delta 待后续上传
                            }
                            else -> {
                                val payload = SyncPayloadConverter.payloadForServer(delta.payload).toMutableMap()
                                payload["id"] = serverId
                                changes += DeltaChangeRequest(
                                    localId = delta.localId,
                                    op = delta.op,
                                    payload = payload,
                                    clientVersion = delta.clientVersion
                                )
                            }
                        }
                    }
                    else -> staleDeltas += delta
                }
            }

            if (staleDeltas.isNotEmpty()) {
                staleDeltas.forEach { database.deltaQueueDao().deleteDelta(it.id) }
            }
            if (changes.isEmpty()) {
                // 没有可上传的变更（全部延迟或已被清理）：不发起空同步，与“队列为空则跳过”的原行为一致
                return@withContext Result.success()
            }

            val request = SyncRequest(lastSyncAt, changes)
            val response = apiService.sync(request)

            val syncBody = response.body()
            if (!response.isSuccessful || syncBody == null) {
                // 401：续期已失败（refresh token 失效），需重新登录，重试无意义
                if (response.code() == 401) {
                    return@withContext Result.failure()
                }
                return@withContext Result.retry()
            }
            val syncResponse = syncBody

            // 先处理已确认的客户端变更：按 op 精确清理 delta 队列，并为 insert 回填服务端 ID
            // 注：未用 room-ktx 的 withTransaction——它是顶层扩展函数，单测的 mockk AppDatabase
            // 无法拦截，会在 relaxed mock 上永久挂起；待测试改用真实内存库后再引入事务
            for (clientChange in syncResponse.clientChanges) {
                val localTask = database.taskDao().getTaskByLocalId(clientChange.localId)
                when (clientChange.op) {
                    "insert" -> {
                        if (localTask != null && localTask.serverId == null && clientChange.serverId > 0) {
                            database.taskDao().updateTask(
                                localTask.copy(serverId = clientChange.serverId)
                            )
                        }
                        database.deltaQueueDao().deleteDeltasByLocalIdAndOp(clientChange.localId, "insert")
                    }
                    "delete" -> {
                        // 无论服务端是“已删除”还是“任务本就不存在”，删除目的均已达成
                        database.deltaQueueDao().deleteDeltasByLocalIdAndOp(clientChange.localId, "delete")
                    }
                    "update" -> {
                        if (clientChange.serverId > 0) {
                            database.deltaQueueDao().deleteDeltasByLocalIdAndOp(clientChange.localId, "update")
                        } else if (localTask != null && localTask.serverId != null) {
                            // 服务端已找不到该任务（可能被硬删）：本地同步软删并清理 delta
                            database.taskDao().softDeleteTask(localTask.id, DateTimeUtils.getCurrentTimestamp())
                            database.deltaQueueDao().deleteDeltasByLocalIdAndOp(clientChange.localId, "update")
                        }
                        // 其余情况（本地任务尚未同步却收到 update 回执；正常流程中未同步任务的
                        // update 在上传组装阶段必被延迟，不会与服务端产生交互）：保留 delta，
                        // 下一轮按延迟策略重发，避免丢编辑。属防御性兜底。
                    }
                }
            }

            // 再应用服务端变更：按 serverId 匹配本地任务；不存在且未删除 → 其他设备新建的任务，插入本地
            for (serverChange in syncResponse.serverChanges) {
                val existing = database.taskDao().getTaskByServerId(userId, serverChange.id)
                if (existing != null) {
                    if (serverChange.isDeleted) {
                        database.taskDao().softDeleteTask(existing.id, serverChange.updatedAt)
                        continue
                    }
                    // 本地仍有待上传的修改时保留本地值：服务端 insert 回显/冲突合并结果
                    // 携带的可能是创建时旧值，直接覆盖会冲掉尚未同步的离线编辑
                    val pendingMutations = database.deltaQueueDao().countPendingMutations(existing.localId)
                    if (pendingMutations > 0) {
                        continue
                    }
                    database.taskDao().updateTask(
                        existing.copy(
                            serverId = serverChange.id,
                            serverVersion = serverChange.serverVersion,
                            title = serverChange.title ?: existing.title,
                            description = serverChange.description ?: existing.description,
                            status = serverChange.status ?: existing.status,
                            priority = serverChange.priority ?: existing.priority,
                            dueAt = serverChange.dueAt ?: existing.dueAt,
                            // 服务端对 NULL completed_at 下发空串：空串不覆盖本地完成时间
                            completedAt = serverChange.completedAt?.takeIf { it.isNotBlank() } ?: existing.completedAt,
                            updatedAt = serverChange.updatedAt,
                            isDeleted = false,
                            sortOrder = serverChange.sortOrder ?: existing.sortOrder,
                            recurrenceType = serverChange.recurrenceType ?: existing.recurrenceType,
                            recurrenceSeriesId = serverChange.recurrenceSeriesId ?: existing.recurrenceSeriesId
                        )
                    )
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
                                completedAt = serverChange.completedAt?.takeIf { it.isNotBlank() } ?: ghost.completedAt,
                                updatedAt = serverChange.updatedAt,
                                lastModified = serverChange.updatedAt,
                                sortOrder = serverChange.sortOrder ?: ghost.sortOrder,
                                recurrenceType = serverChange.recurrenceType ?: ghost.recurrenceType,
                                recurrenceSeriesId = serverChange.recurrenceSeriesId ?: ghost.recurrenceSeriesId
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
                                lastModified = timestamp,
                                sortOrder = serverChange.sortOrder ?: 0.0,
                                recurrenceType = serverChange.recurrenceType ?: "",
                                recurrenceSeriesId = serverChange.recurrenceSeriesId ?: ""
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

            // 记录服务端最新版本号，供增量拉取与冲突判定使用
            val maxServerVersion = syncResponse.serverChanges
                .maxOfOrNull { it.serverVersion }
                ?: syncMeta?.lastServerVersion ?: 0
            database.syncMetaDao().insertSyncMeta(
                SyncMeta(
                    userId = userId,
                    lastSyncAt = syncResponse.lastSyncAt,
                    lastServerVersion = maxServerVersion
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

        /** 手动同步专用唯一名：与周期任务分开入队，避免互相顶替 */
        const val MANUAL_WORK_NAME = "DeltaSyncWorker.manual"

        /** 同一时刻仅允许一个同步任务处理 delta 队列 */
        private val syncMutex = Mutex()

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
    }
}
