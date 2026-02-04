package com.todoapp.ui.tasks

import android.content.Context
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.work.Constraints
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkManager
import com.todoapp.R
import com.todoapp.data.repository.TaskRepository
import com.todoapp.data.sync.DeltaSyncWorker
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
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

    val tasks = taskRepository.allTasks

    private val _syncState = MutableStateFlow<SyncState>(SyncState.Idle)
    val syncState: StateFlow<SyncState> = _syncState.asStateFlow()

    private val _connectionState = MutableStateFlow(true)
    val connectionState: StateFlow<Boolean> = _connectionState.asStateFlow()

    private val connectivityManager: ConnectivityManager by lazy {
        context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
    }

    private var networkCallback: ConnectivityManager.NetworkCallback? = null

    init {
        startNetworkMonitoring()
    }

    private fun startNetworkMonitoring() {
        val networkRequest = NetworkRequest.Builder()
            .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            .addTransportType(NetworkCapabilities.TRANSPORT_WIFI)
            .addTransportType(NetworkCapabilities.TRANSPORT_CELLULAR)
            .build()

        networkCallback = object : ConnectivityManager.NetworkCallback() {
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

        connectivityManager.registerNetworkCallback(networkRequest, networkCallback!!)
        updateConnectionStatus()
    }

    private fun updateConnectionStatus() {
        val network = connectivityManager.activeNetwork
        val capabilities = connectivityManager.getNetworkCapabilities(network)
        _connectionState.value = capabilities?.hasCapability(
            NetworkCapabilities.NET_CAPABILITY_INTERNET
        ) == true
    }

    fun toggleTaskCompletion(task: com.todoapp.data.local.Task) {
        viewModelScope.launch {
            taskRepository.toggleTaskCompletion(task)
                .onError { e ->
                    _syncState.value = SyncState.Error(
                        context.getString(R.string.sync_update_failed, e.message)
                    )
                }
        }
    }

    fun deleteTask(task: com.todoapp.data.local.Task) {
        viewModelScope.launch {
            taskRepository.deleteTask(task)
                .onError { e ->
                    _syncState.value = SyncState.Error(
                        context.getString(R.string.sync_delete_failed, e.message)
                    )
                }
        }
    }

    fun triggerManualSync() {
        viewModelScope.launch {
            _syncState.value = SyncState.Syncing

            try {
                val workRequest = OneTimeWorkRequestBuilder<DeltaSyncWorker>()
                    .setConstraints(
                        Constraints.Builder()
                            .setRequiredNetworkType(NetworkType.CONNECTED)
                            .build()
                    )
                    .build()

                WorkManager.getInstance(context).enqueue(workRequest)

                kotlinx.coroutines.delay(2000)
                _syncState.value = SyncState.Success

            } catch (e: Exception) {
                _syncState.value = SyncState.Error(
                    context.getString(R.string.sync_failed, e.message)
                )
            }
        }
    }

    override fun onCleared() {
        super.onCleared()
        networkCallback?.let {
            connectivityManager.unregisterNetworkCallback(it)
        }
        networkCallback = null
    }
}
