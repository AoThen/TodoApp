package com.todoapp.ui.tasks

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.todoapp.TodoApp
import com.todoapp.data.local.entities.Task
import com.todoapp.data.local.dao.TaskDao
import com.todoapp.data.remote.RetrofitClient
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import java.util.*

sealed class SyncState {
    object Idle : SyncState()
    object Syncing : SyncState()
    object Success : SyncState()
    data class Error(val message: String) : SyncState()
}

class TaskListViewModel(application: Application) : AndroidViewModel(application) {
    
    private val context = getApplication<TodoApp>().applicationContext
    private val database = (getApplication() as TodoApp).database
    private val taskDao: TaskDao = database.taskDao()
    
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
            val updatedTask = task.copy(
                status = if (task.status == "completed") "todo" else "completed",
                updatedAt = getCurrentTimestamp(),
                lastModified = getCurrentTimestamp()
            )
            
            if (task.status != "completed") {
                updatedTask.completedAt = getCurrentTimestamp()
            }
            
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
                // Use existing DeltaSyncWorker
                val workRequest = androidx.work.OneTimeWorkRequestBuilder<com.todoapp.data.sync.DeltaSyncWorker>()
                    .setConstraints(
                        androidx.work.Constraints.Builder()
                            .setRequiredNetworkType(androidx.work.NetworkType.CONNECTED)
                            .build()
                    )
                    .build()
                
                androidx.work.WorkManager.getInstance(context).enqueue(workRequest)
                
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
                    // Simple connectivity check
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
            val delta = com.todoapp.data.local.entities.DeltaChange(
                localId = task.localId,
                op = operation,
                payload = com.google.gson.Gson().toJson(task),
                clientVersion = (task.serverVersion ?: 0) + 1,
                timestamp = getCurrentTimestamp()
            )
            
            database.deltaQueueDao().insertDelta(delta)
        } catch (e: Exception) {
            // Handle error
        }
    }

    private fun getCurrentTimestamp(): String {
        return java.text.SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'", Locale.getDefault())
            .format(Date())
    }
}