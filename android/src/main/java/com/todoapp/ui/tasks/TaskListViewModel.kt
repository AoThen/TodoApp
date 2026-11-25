package com.todoapp.ui.tasks

import android.content.Context
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.work.Constraints
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkInfo
import androidx.work.WorkManager
import com.todoapp.R
import com.todoapp.config.AppConfig
import com.todoapp.data.local.Task
import com.todoapp.data.repository.TaskRepository
import com.todoapp.data.sync.DeltaSyncWorker
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.launch
import java.util.UUID
import javax.inject.Inject

sealed class SyncState {
    object Idle : SyncState()
    object Syncing : SyncState()
    object Success : SyncState()
    data class Error(val message: String) : SyncState()
}

@HiltViewModel
class TaskListViewModel @Inject constructor(
    @ApplicationContext private val context: Context,
    private val taskRepository: TaskRepository
) : ViewModel() {

    private val _searchQuery = MutableStateFlow("")

    /** 当前搜索词（供界面区分「暂无任务」与「无匹配结果」） */
    val searchQuery: StateFlow<String> = _searchQuery.asStateFlow()

    // 排序模式持久化：默认创建时间倒序（与历史行为一致），手动排序需用户显式切换
    private val sortPrefs = context.getSharedPreferences("task_ui", Context.MODE_PRIVATE)
    private val _sortMode = MutableStateFlow(
        sortPrefs.getString(SORT_MODE_KEY, SORT_MODE_CREATED) ?: SORT_MODE_CREATED
    )
    val sortMode: StateFlow<String> = _sortMode.asStateFlow()

    fun setSortMode(mode: String) {
        _sortMode.value = mode
        sortPrefs.edit().putString(SORT_MODE_KEY, mode).apply()
    }

    /** 任务流：按搜索词过滤后按排序模式排列 */
    val tasks = combine(taskRepository.getAllTasks(), _searchQuery, _sortMode) { list, query, mode ->
        sortTasks(filterTasks(list, query), mode)
    }

    /** 拖拽释放：按移动后列表中该位置的相邻 sort_order 中点持久化，仅被拖任务产生一条 delta */
    fun persistReorderAt(list: List<Task>, position: Int) {
        val moved = list.getOrNull(position) ?: return
        val newSortOrder = computeSortOrderForPosition(list, position) ?: return
        viewModelScope.launch {
            taskRepository.persistSortOrder(moved, newSortOrder)
                .onError { e ->
                    _syncState.value = SyncState.Error(
                        context.getString(R.string.sync_update_failed, e.message)
                    )
                }
        }
    }

    fun setSearchQuery(query: String) {
        _searchQuery.value = query.trim()
    }

    private val _syncState = MutableStateFlow<SyncState>(SyncState.Idle)
    val syncState: StateFlow<SyncState> = _syncState.asStateFlow()

    private val _connectionState = MutableStateFlow(true)
    val connectionState: StateFlow<Boolean> = _connectionState.asStateFlow()

    private val connectivityManager: ConnectivityManager by lazy {
        context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
    }

    private var networkCallback: ConnectivityManager.NetworkCallback? = null
    private var syncObservationJob: Job? = null

    init {
        startNetworkMonitoring()
    }

    private fun startNetworkMonitoring() {
        val networkRequest = NetworkRequest.Builder()
            .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            .build()

        val callback = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) {
                _connectionState.value = true
            }

            override fun onLost(network: Network) {
                _connectionState.value = false
            }

            override fun onCapabilitiesChanged(
                network: Network,
                networkCapabilities: NetworkCapabilities
            ) {
                val hasInternet = networkCapabilities.hasCapability(
                    NetworkCapabilities.NET_CAPABILITY_INTERNET
                )
                _connectionState.value = hasInternet
            }
        }

        networkCallback = callback
        connectivityManager.registerNetworkCallback(networkRequest, callback)
        updateConnectionStatus()
    }

    private fun updateConnectionStatus() {
        val network = connectivityManager.activeNetwork
        val capabilities = connectivityManager.getNetworkCapabilities(network)
        _connectionState.value = capabilities?.hasCapability(
            NetworkCapabilities.NET_CAPABILITY_INTERNET
        ) == true
    }

    fun toggleTaskCompletion(task: Task) {
        viewModelScope.launch {
            taskRepository.toggleTaskCompletion(task)
                .onError { e ->
                    _syncState.value = SyncState.Error(
                        context.getString(R.string.sync_update_failed, e.message)
                    )
                }
        }
    }

    fun triggerManualSync(): UUID? {
        // 游客模式无登录态，同步无意义；保持空闲态避免进度条误转
        if (AppConfig.isGuestMode(context)) {
            _syncState.value = SyncState.Idle
            return null
        }

        _syncState.value = SyncState.Syncing

        return try {
            val workRequest = OneTimeWorkRequestBuilder<DeltaSyncWorker>()
                .setConstraints(
                    Constraints.Builder()
                        .setRequiredNetworkType(NetworkType.CONNECTED)
                        .build()
                )
                .build()

            // 唯一名 + KEEP：手动同步进行中时重复触发不重复入队；
            // 与周期任务的并发互斥由 Worker 内部的同步锁兜底
            WorkManager.getInstance(context).enqueueUniqueWork(
                DeltaSyncWorker.MANUAL_WORK_NAME,
                ExistingWorkPolicy.KEEP,
                workRequest
            )
            observeSyncWork()
            workRequest.id
        } catch (e: Exception) {
            _syncState.value = SyncState.Error(
                context.getString(R.string.sync_failed, e.message)
            )
            null
        }
    }

    /** 观察手动同步任务状态，结束时回写 syncState，避免进度条永久转动 */
    private fun observeSyncWork() {
        syncObservationJob?.cancel()
        syncObservationJob = viewModelScope.launch {
            WorkManager.getInstance(context)
                .getWorkInfosForUniqueWorkFlow(DeltaSyncWorker.MANUAL_WORK_NAME)
                .collect { infos ->
                    val info = infos.firstOrNull() ?: return@collect
                    when (info.state) {
                        WorkInfo.State.SUCCEEDED -> {
                            _syncState.value = SyncState.Success
                            syncObservationJob?.cancel()
                        }
                        WorkInfo.State.FAILED -> {
                            _syncState.value = SyncState.Error(
                                context.getString(R.string.sync_failed, "同步失败")
                            )
                            syncObservationJob?.cancel()
                        }
                        else -> {
                            // ENQUEUED / RUNNING 保持 Syncing
                        }
                    }
                }
        }
    }

    override fun onCleared() {
        super.onCleared()
        networkCallback?.let {
            connectivityManager.unregisterNetworkCallback(it)
        }
        networkCallback = null
        syncObservationJob?.cancel()
        syncObservationJob = null
    }

    companion object {
        const val SORT_MODE_KEY = "task_sort_mode"
        const val SORT_MODE_CREATED = "created_at"
        const val SORT_MODE_MANUAL = "manual"
        const val SORT_MODE_UPDATED = "updated_at"
        const val SORT_MODE_PRIORITY = "priority"

        /** 搜索过滤：空词返回全量；命中标题或描述（忽略大小写） */
        fun filterTasks(list: List<Task>, query: String): List<Task> {
            if (query.isBlank()) return list
            return list.filter { task ->
                task.title.contains(query, ignoreCase = true) ||
                    task.description.contains(query, ignoreCase = true)
            }
        }

        /** 排序：manual 按 sortOrder 升序（同值回退创建时间倒序，与后端 manual 一致） */
        fun sortTasks(list: List<Task>, mode: String): List<Task> = when (mode) {
            SORT_MODE_MANUAL -> list.sortedWith(
                compareBy<Task> { it.sortOrder }.thenByDescending { it.createdAt }
            )
            SORT_MODE_UPDATED -> list.sortedByDescending { it.updatedAt }
            SORT_MODE_PRIORITY -> list.sortedBy { priorityRank(it.priority) }
            else -> list.sortedByDescending { it.createdAt }
        }

        /**
         * 拖拽落点 sort_order 计算：取移动后列表中 position 位置的相邻任务中点；边界 ±1000；越界返回 null。
         * 注意 list 必须是"已完成移动"的列表（与 ItemTouchHelper.onMove 提交后的 currentList 一致），
         * 被拖任务即 list[position]，不可再模拟一次移动。
         */
        fun computeSortOrderForPosition(list: List<Task>, position: Int): Double? {
            if (position !in list.indices) return null
            val prev = list.getOrNull(position - 1)
            val follow = list.getOrNull(position + 1)
            return when {
                prev != null && follow != null -> (prev.sortOrder + follow.sortOrder) / 2.0
                prev != null -> prev.sortOrder + 1000.0
                follow != null -> follow.sortOrder - 1000.0
                else -> 0.0
            }
        }

        private fun priorityRank(priority: String): Int = when (priority) {
            "high" -> 1
            "medium" -> 2
            else -> 3
        }
    }
}
