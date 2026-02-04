package com.todoapp.ui.tasks

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewModelScope
import com.todoapp.TodoApp
import com.todoapp.data.local.Task
import com.todoapp.data.remote.ApiService
import com.todoapp.data.remote.RetrofitClient
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import java.util.UUID

sealed class AddTaskState {
    object Idle : AddTaskState()
    object Loading : AddTaskState()
    data class Success(val task: Task) : AddTaskState()
    data class Error(val message: String) : AddTaskState()
}

data class AddTaskFormState(
    val titleError: String? = null,
    val descriptionError: String? = null,
    val isFormValid: Boolean = false
)

class AddTaskViewModel(
    application: Application,
    private val taskDao: com.todoapp.data.local.TaskDao,
    private val deltaQueueDao: com.todoapp.data.local.DeltaQueueDao
) : AndroidViewModel(application) {

    private val context = getApplication<TodoApp>().applicationContext
    private val apiService: ApiService = RetrofitClient.getApiService(context)

    private val _uiState = MutableStateFlow<AddTaskState>(AddTaskState.Idle)
    val uiState: StateFlow<AddTaskState> = _uiState.asStateFlow()

    private val _formState = MutableStateFlow(AddTaskFormState())
    val formState: StateFlow<AddTaskFormState> = _formState.asStateFlow()

    private var currentTitle: String = ""
    private var currentDescription: String = ""
    private var currentDueDate: String = ""
    private var currentStatus: String = "todo"
    private var currentPriority: String = "medium"

    fun validateTitle(title: String) {
        currentTitle = title.trim()
        val error = when {
            currentTitle.isEmpty() -> "标题不能为空"
            currentTitle.length > 200 -> "标题不能超过200个字符"
            else -> null
        }
        _formState.value = _formState.value.copy(titleError = error)
    }

    fun validateDescription(description: String) {
        currentDescription = description.trim()
        val error = when {
            currentDescription.length > 2000 -> "描述不能超过2000个字符"
            else -> null
        }
        _formState.value = _formState.value.copy(descriptionError = error)
    }

    fun validateDueDate(dueDate: String) {
        currentDueDate = dueDate.trim()
    }

    fun updateStatus(status: String) {
        currentStatus = status
    }

    fun updatePriority(priority: String) {
        currentPriority = priority
    }

    fun validateForm() {
        validateTitle(currentTitle)
        validateDescription(currentDescription)
        validateDueDate(currentDueDate)
    }

    fun createTask() {
        viewModelScope.launch {
            _uiState.value = AddTaskState.Loading
            try {
                if (!_formState.value.isFormValid) {
                    _uiState.value = AddTaskState.Error("请修正表单中的错误")
                    return@launch
                }

                val now = System.currentTimeMillis()
                val localId = UUID.randomUUID().toString()
                val userId = "current-user"

                val task = Task(
                    localId = localId,
                    userId = userId,
                    title = currentTitle,
                    description = currentDescription,
                    status = currentStatus,
                    priority = currentPriority,
                    dueAt = if (currentDueDate.isNotBlank()) "${currentDueDate}T00:00:00Z" else null,
                    createdAt = now.toString(),
                    updatedAt = now.toString(),
                    lastModified = now.toString()
                )

                val taskId = taskDao.insertTask(task)

                val deltaChange = com.todoapp.data.local.DeltaChange(
                    localId = localId,
                    op = "insert",
                    payload = """
                        {
                            "local_id": "$localId",
                            "title": "${currentTitle}",
                            "description": "${currentDescription}",
                            "status": "$currentStatus",
                            "priority": "$currentPriority",
                            "due_at": ${if (currentDueDate.isNotBlank()) "\"${currentDueDate}T00:00:00Z\"" else "null"}
                        }
                    """.trimIndent(),
                    clientVersion = 1,
                    timestamp = now.toString()
                )

                deltaQueueDao.insertDelta(deltaChange)

                _uiState.value = AddTaskState.Success(task.copy(id = taskId))

                resetForm()
            } catch (e: Exception) {
                _uiState.value = AddTaskState.Error("创建任务失败: ${e.message}")
            }
        }
    }

    fun resetForm() {
        currentTitle = ""
        currentDescription = ""
        currentDueDate = ""
        currentStatus = "todo"
        currentPriority = "medium"
        _formState.value = AddTaskFormState(titleError = null, descriptionError = null, isFormValid = false)
    }

    fun getCurrentFormValues(): Map<String, String> {
        return mapOf(
            "title" to currentTitle,
            "description" to currentDescription,
            "due_date" to currentDueDate,
            "status" to currentStatus,
            "priority" to currentPriority
        )
    }

    class Factory(
        private val application: Application,
        private val taskDao: com.todoapp.data.local.TaskDao,
        private val deltaQueueDao: com.todoapp.data.local.DeltaQueueDao
    ) : ViewModelProvider.Factory {
        override fun <T : androidx.lifecycle.ViewModel> create(modelClass: Class<T>): T {
            @Suppress("UNCHECKED_CAST")
            return AddTaskViewModel(application, taskDao, deltaQueueDao) as T
        }
    }
}
