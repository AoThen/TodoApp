package com.todoapp.data.sync

import android.content.Context
import android.util.Log
import com.todoapp.data.local.AppDatabase
import com.todoapp.data.local.Task
import com.todoapp.data.remote.RetrofitClient
import com.todoapp.data.remote.TaskResponse
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject

/**
 * 游客任务登录对账：登录后、首次上传前，把内容与服务端已有任务完全一致的
 * 离线新增任务直接"认领"（回填服务端 ID 并撤销其 insert 上传指令），
 * 避免同内容任务在服务端/本机重复出现。
 *
 * 失败语义：任一环节异常均整体跳过对账，回退为原样上传，绝不阻塞登录流程。
 */
class GuestTaskReconciler @Inject constructor(
    @ApplicationContext private val context: Context,
    private val database: AppDatabase
) {

    /** @return 认领合并的任务条数 */
    suspend fun reconcile(): Int {
        return try {
            val candidates = database.deltaQueueDao().getAllDeltas()
                .filter { it.op == "insert" }
                .mapNotNull { delta ->
                    database.taskDao().getTaskByLocalId(delta.localId)?.takeIf { it.serverId == null }
                }
                // 防御：同一任务残留多条 insert delta 时只对账一次
                .distinctBy { it.localId }
            // 无待上传的离线新增时不必请求服务端
            if (candidates.isEmpty()) return 0

            val serverTasks = fetchAllServerTasks() ?: return 0
            if (serverTasks.isEmpty()) return 0

            val claimedServerIds = mutableSetOf<Long>()
            var merged = 0
            for (task in candidates) {
                val match = serverTasks.firstOrNull { server ->
                    server.id !in claimedServerIds && isSameContent(task, server)
                } ?: continue

                claimedServerIds += match.id
                // 认领服务端身份后，本地任务不再作为新任务上传；
                // 游客期间的后续修改 delta 保留，上传时按 localId 注入当前 serverId
                database.taskDao().updateTask(task.copy(serverId = match.id))
                database.deltaQueueDao().deleteDeltasByLocalIdAndOp(task.localId, "insert")
                merged++
            }
            merged
        } catch (e: Exception) {
            Log.w(TAG, "游客任务对账失败，回退为原样上传", e)
            0
        }
    }

    /** 分页拉取服务端全量任务；任何失败返回 null 表示放弃对账 */
    private suspend fun fetchAllServerTasks(): List<TaskResponse>? {
        return try {
            val all = mutableListOf<TaskResponse>()
            var page = 1
            while (true) {
                // 动态解析：切换服务器后立刻指向新地址
                val response = RetrofitClient.getApiService(context).getTasksPage(page, PAGE_SIZE)
                val body = response.body()
                if (!response.isSuccessful || body == null) return null
                val data = body.data ?: return null
                all += data.tasks
                // 服务端 pages 异常时的本地保护：限制最大页数，空页提前终止
                val pages = (data.pagination?.pages ?: 1).coerceIn(1, MAX_PAGES)
                if (data.tasks.isEmpty() || page >= pages) break
                page++
            }
            all
        } catch (e: Exception) {
            Log.w(TAG, "拉取服务端任务失败，跳过对账", e)
            null
        }
    }

    /** 内容全等才认定同一条：标题、描述、状态、优先级、截止时间（空串与 null 等价） */
    private fun isSameContent(task: Task, server: TaskResponse): Boolean {
        return task.title == server.title &&
            task.description.orEmpty() == server.description.orEmpty() &&
            task.status == server.status &&
            task.priority == (server.priority ?: "medium") &&
            task.dueAt.orEmpty() == server.dueAt.orEmpty()
    }

    companion object {
        private const val TAG = "GuestTaskReconciler"
        private const val PAGE_SIZE = 100
        private const val MAX_PAGES = 1000
    }
}
