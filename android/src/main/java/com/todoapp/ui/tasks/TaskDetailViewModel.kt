package com.todoapp.ui.tasks

import androidx.fragment.app.Fragment
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.viewmodelScope
import com.todoapp.TodoApp
import com.todoapp.data.Task
import com.todoapp.data.local.AppDatabase
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

sealed class TaskDetailState {
    object Idle : TaskDetailState()
    object Loading : TaskDetailState()
    data class Success(val task: Task) : TaskDetailState()
    data class Error(val message: String) : TaskDetailState()
}

data class TaskFormState(
    val titleError: String? = null,
    val descriptionError: String? = null,
    val isFormValid: Boolean = false
)

class TaskDetailViewModel(
    private val taskDao: com.todoapp.data.local.TaskDao
) : androidx.lifecycle.ViewModel() {

    private val _uiState = MutableStateFlow<TaskDetailState>(TaskDetailState.Idle)
    val uiState: StateFlow<TaskDetailState> = _uiState.asStateFlow()

    private val _formState = MutableStateFlow(TaskFormState())
    val formState: StateFlow<TaskFormState> = _formState.asStateFlow()

    private var originalTask: Task? = null
    var hasUnsavedChanges: Boolean = false
        private set

    private var currentTitle: String = ""
    private var currentDescription: String = ""
    private var currentDueDate: String = ""
    private var currentStatus: String = "todo"
    private var currentPriority: String = "medium"

    fun loadTask(taskId: String) {
        viewModelScope.launch {
            _uiState.value = TaskDetailState.Loading
            try {
                val task = taskDao.getTaskById(taskId.toLongOrNull() ?: 0)
                if (task != null) {
                    originalTask = task.copy()
                    currentTitle = task.title
                    currentDescription = task.description
                    currentDueDate = task.dueAt.substringBefore("T").takeIf { task.dueAt.isNotBlank() } ?: ""
                    currentStatus = task.status
                    currentPriority = task.priority
                    hasUnsavedChanges = false
                    validateForm(currentTitle, currentDescription, currentDueDate)
                    _uiState.value = TaskDetailState.Success(task)
                } else {
                    _uiState.value = TaskDetailState.Error("任务不存在")
                }
            } catch (e: Exception) {
                _uiState.value = TaskDetailState.Error("加载失败: ${e.message}")
            }
        }
    }

    fun validateTitle(title: String) {
        currentTitle = title
        val error = when {
            title.isBlank() -> "标题不能为空"
            title.length > 200 -> "标题不能超过200个字符"
            else -> null
        }
        _formState.value = _formState.value.copy(titleError = error)
        checkUnsavedChanges()
    }

    fun validateDescription(description: String) {
        currentDescription = description
        val error = when {
            description.length > 2000 -> "描述不能超过2000个字符"
            else -> null
        }
        _formState.value = _formState.value.copy(descriptionError = error)
        checkUnsavedChanges()
    }

    fun validateDueDate(date: String) {
        currentDueDate = date
        checkUnsavedChanges()
    }

    fun updateStatus(status: String) {
        currentStatus = status
        checkUnsavedChanges()
    }

    fun updatePriority(priority: String) {
        currentPriority = priority
        checkUnsavedChanges()
    }

    fun validateForm(title: String, description: String, dueDate: String) {
        validateTitle(title)
        validateDescription(description)
        validateDueDate(dueDate)
    }

    private fun checkUnsavedChanges() {
        hasUnsavedChanges = originalTask != null && (
            currentTitle != originalTask!!.title ||
            currentDescription != originalTask!!.description ||
            currentDueDate != originalTask!!.dueAt.substringBefore("T").takeIf { originalTask!!.dueAt.isNotBlank() } ?: "" ||
            currentStatus != originalTask!!.status ||
            currentPriority != originalTask!!.priority
        )
    }

    fun updateTask() {
        viewModelScope.launch {
            _uiState.value = TaskDetailState.Loading
            try {
                val task = getCurrentTask() ?: return@launch
                if (!_formState.value.isFormValid) {
                    _uiState.value = TaskDetailState.Error("请修正表单中的错误")
                    return@launch
                }

                val now = System.currentTimeMillis()

                val updated = task.copy(
                    title = currentTitle,
                    description = currentDescription,
                    status = currentStatus,
                    priority = currentPriority,
                    dueAt = if (currentDueDate.isNotBlank()) "$currentDueDateT00:00:00Z" else task.dueAt,
                    updatedAt = now.toString(),
                    serverVersion = (task.serverVersion ?: 0) + 1,
                    lastModified = now.toString()
                )

                taskDao.updateTask(updated)
                originalTask = updated.copy()
                hasUnsavedChanges = false
                _uiState.value = TaskDetailState.Success(updated)
            } catch (e: Exception) {
                _uiState.value = TaskDetailState.Error("更新失败: ${e.message}")
            }
        }
    }

    fun deleteTask(taskId: String) {
        viewModelScope.launch {
            _uiState.value = TaskDetailState.Loading
            try {
                taskDao.softDeleteTask(taskId.toLongOrNull() ?: 0, System.currentTimeMillis().toString())
                _uiState.value = TaskDetailState.Success(getCurrentTask() ?: return@launch)
            } catch (e: Exception) {
                _uiState.value = TaskDetailState.Error("删除失败: ${e.message}")
            }
        }
    }

    fun revertChanges() {
        originalTask?.let { task ->
            currentTitle = task.title
            currentDescription = task.description
            currentDueDate = task.dueAt.substringBefore("T").takeIf { task.dueAt.isNotBlank() } ?: ""
            currentStatus = task.status
            currentPriority = task.priority
            hasUnsavedChanges = false
            _formState.value = TaskFormState(titleError = null, descriptionError = null, isFormValid = true)
        }
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

    fun getCurrentTask(): Task? {
        val state = _uiState.value
        if (state is TaskDetailState.Success) {
            return state.task
        }
        return null
    }

    class Factory(private val taskDao: com.todoapp.data.local.TaskDao) : ViewModelProvider.Factory {
        override fun <T : ViewModel> create(modelClass: Class): T {
            @Suppress("UNCHECKED_CAST")
            return TaskDetailViewModel(taskDao) as T
        }
    }
}
