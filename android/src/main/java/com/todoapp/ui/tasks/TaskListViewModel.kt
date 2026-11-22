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

    /** 任务流：按搜索词过滤标题/描述（空词返回全量） */
    val tasks = combine(taskRepository.getAllTasks(), _searchQuery) { list, query ->
        filterTasks(list, query)
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
        /** 搜索过滤：空词返回全量；命中标题或描述（忽略大小写） */
        fun filterTasks(list: List<Task>, query: String): List<Task> {
            if (query.isBlank()) return list
            return list.filter { task ->
                task.title.contains(query, ignoreCase = true) ||
                    task.description.contains(query, ignoreCase = true)
            }
        }
    }
}
