package com.todoapp.data.sync

import android.content.Context
import androidx.work.*
import com.todoapp.data.local.AppDatabase
import com.todoapp.data.local.DeltaChange
import com.todoapp.data.local.SyncMeta
import com.todoapp.data.remote.DeltaChangeRequest
import com.todoapp.data.remote.RetrofitClient
import com.todoapp.data.remote.SyncRequest
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.util.concurrent.TimeUnit

class DeltaSyncWorker(
    context: Context,
    params: WorkerParameters
) : CoroutineWorker(context, params) {

    private val database = AppDatabase.getInstance(context)
    private val apiService = RetrofitClient.getApiService(context)

    override suspend fun doWork(): Result = withContext(Dispatchers.IO) {
        try {
            val pendingDeltas = database.deltaQueueDao().getAllDeltas()
            if (pendingDeltas.isEmpty()) {
                return@withContext Result.success()
            }

            val syncMeta = database.syncMetaDao().getSyncMeta("current-user")
            val lastSyncAt = syncMeta?.lastSyncAt ?: ""

            val changes: List<DeltaChangeRequest> = pendingDeltas.map { delta ->
                DeltaChangeRequest(
                    localId = delta.localId,
                    op = delta.op,
                    payload = mapOf("data" to delta.payload),
                    clientVersion = delta.clientVersion
                )
            }

            val request = SyncRequest(lastSyncAt, changes)
            val response = apiService.sync(request)

            if (!response.isSuccessful || response.body() == null) {
                return@withContext Result.retry()
            }

            val syncResponse = response.body()!!

            for (serverChange in syncResponse.serverChanges) {
                val task = database.taskDao().getTaskById(serverChange.id)
                if (task != null) {
                    database.taskDao().updateTask(
                        task.copy(
                            serverId = serverChange.id,
                            serverVersion = serverChange.serverVersion,
                            title = serverChange.title,
                            updatedAt = serverChange.updatedAt,
                            isDeleted = serverChange.isDeleted
                        )
                    )
                }
            }

            for (clientChange in syncResponse.clientChanges) {
                val delta = pendingDeltas.find { it.localId == clientChange.localId }
                delta?.let {
                    database.deltaQueueDao().deleteDelta(it.id)
                }
            }

            for (conflict in syncResponse.conflicts) {
                database.conflictDao().insertConflict(
                    com.todoapp.data.local.Conflict(
                        localId = conflict.localId,
                        serverId = conflict.serverId,
                        reason = conflict.reason,
                        options = conflict.options.joinToString(","),
                        createdAt = java.time.Instant.now().toString()
                    )
                )
            }

            database.syncMetaDao().insertSyncMeta(
                SyncMeta(
                    userId = "current-user",
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

    companion object {
        const val WORK_NAME = "DeltaSyncWorker"

        fun schedule(context: Context) {
            val constraints = Constraints.Builder()
                .setRequiredNetworkType(NetworkType.CONNECTED)
                .build()

            val request = PeriodicWorkRequestBuilder<DeltaSyncWorker>(
                15, TimeUnit.MINUTES
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
