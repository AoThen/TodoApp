package com.todoapp.ui.tasks

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import androidx.work.Constraints
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkManager
import com.todoapp.TodoApp
import com.todoapp.data.local.AppDatabase
import com.todoapp.data.local.DeltaChange
import com.todoapp.data.local.Task
import com.todoapp.data.remote.RetrofitClient
import com.todoapp.data.sync.DeltaSyncWorker
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.UUID

sealed class SyncState {
    object Idle : SyncState()
    object Syncing : SyncState()
    object Success : SyncState()
    data class Error(val message: String) : SyncState()
}

class TaskListViewModel(application: Application) : AndroidViewModel(application) {
    
    private val context = getApplication<TodoApp>().applicationContext
    private val database = (getApplication() as TodoApp).database
    private val taskDao = database.taskDao()
    private val deltaQueueDao = database.deltaQueueDao()
    
    val tasks = taskDao.getAllTasks()
    
    private val _syncState = MutableStateFlow<SyncState>(SyncState.Idle)
    val syncState: StateFlow<SyncState> = _syncState.asStateFlow()
    
    private val _connectionState = MutableStateFlow(true)
    val connectionState: StateFlow<Boolean> = _connectionState.asStateFlow()

    init {
        checkConnectionStatus()
    }

    fun toggleTaskCompletion(task: Task) {
        viewModelScope.launch {
            val isCompleting = task.status != "completed"
            val updatedTask = task.copy(
                status = if (isCompleting) "completed" else "todo",
                updatedAt = getCurrentTimestamp(),
                lastModified = getCurrentTimestamp(),
                completedAt = if (isCompleting) getCurrentTimestamp() else null
            )
            
            taskDao.updateTask(updatedTask)
            
            // Add to delta queue for sync
            addToDeltaQueue("update", updatedTask)
        }
    }

    fun deleteTask(task: Task) {
        viewModelScope.launch {
            val deletedTask = task.copy(
                isDeleted = true,
                updatedAt = getCurrentTimestamp(),
                lastModified = getCurrentTimestamp()
            )
            
            taskDao.updateTask(deletedTask)
            
            // Add to delta queue for sync
            addToDeltaQueue("delete", deletedTask)
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
                
                // Wait a moment and then check status (simplified)
                kotlinx.coroutines.delay(2000)
                _syncState.value = SyncState.Success
                
            } catch (e: Exception) {
                _syncState.value = SyncState.Error("同步失败: ${e.message}")
            }
        }
    }

    private fun checkConnectionStatus() {
        viewModelScope.launch {
            while (true) {
                try {
                    val url = java.net.URL(RetrofitClient.getBaseUrl())
                    val connection = url.openConnection() as java.net.HttpURLConnection
                    connection.requestMethod = "HEAD"
                    connection.connectTimeout = 5000
                    connection.readTimeout = 5000
                    val responseCode = connection.responseCode
                    _connectionState.value = responseCode in 200..399
                    connection.disconnect()
                } catch (e: Exception) {
                    _connectionState.value = false
                }
                
                kotlinx.coroutines.delay(30000) // Check every 30 seconds
            }
        }
    }

    private suspend fun addToDeltaQueue(operation: String, task: Task) {
        try {
            val delta = DeltaChange(
                localId = task.localId,
                op = operation,
                payload = com.google.gson.Gson().toJson(task),
                clientVersion = (task.serverVersion ?: 0) + 1,
                timestamp = getCurrentTimestamp()
            )
            
            deltaQueueDao.insertDelta(delta)
        } catch (e: Exception) {
            // Handle error
        }
    }

    private fun getCurrentTimestamp(): String {
        return SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'", Locale.getDefault())
            .format(Date())
    }
}