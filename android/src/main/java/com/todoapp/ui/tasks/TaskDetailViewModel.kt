package com.todoapp.ui.tasks

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.todoapp.R
import com.todoapp.data.local.Task
import com.todoapp.data.local.TaskDao
import com.todoapp.data.repository.TaskRepository
import com.todoapp.utils.DateTimeUtils
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

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

@HiltViewModel
class TaskDetailViewModel @Inject constructor(
    private val taskDao: TaskDao,
    private val taskRepository: TaskRepository
) : ViewModel() {

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

    fun loadTask(taskId: String, context: android.content.Context) {
        viewModelScope.launch {
            _uiState.value = TaskDetailState.Loading

            taskRepository.getTaskById(taskId.toLongOrNull() ?: 0)
                .onSuccess { task ->
                    originalTask = task.copy()
                    currentTitle = task.title
                    currentDescription = task.description
                    currentDueDate = task.dueAt?.substringBefore("T")?.takeIf { it.isNotBlank() } ?: ""
                    currentStatus = task.status
                    currentPriority = task.priority
                    hasUnsavedChanges = false
                    validateForm(currentTitle, currentDescription, currentDueDate, context)
                    _uiState.value = TaskDetailState.Success(task)
                }
                .onError { e ->
                    val errorMessage = when {
                        e is NoSuchElementException -> context.getString(R.string.task_not_found)
                        else -> context.getString(R.string.task_load_failed, e.message)
                    }
                    _uiState.value = TaskDetailState.Error(errorMessage)
                }
        }
    }

    fun validateTitle(title: String, context: android.content.Context) {
        currentTitle = title
        val error = when {
            title.isBlank() -> context.getString(R.string.task_title_empty)
            title.length > 200 -> context.getString(R.string.task_title_too_long)
            else -> null
        }
        _formState.value = _formState.value.copy(titleError = error)
        checkUnsavedChanges()
    }

    fun validateDescription(description: String, context: android.content.Context) {
        currentDescription = description
        val error = when {
            description.length > 2000 -> context.getString(R.string.task_description_too_long)
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

    fun validateForm(title: String, description: String, dueDate: String, context: android.content.Context) {
        validateTitle(title, context)
        validateDescription(description, context)
        validateDueDate(dueDate)
    }

    private fun checkUnsavedChanges() {
        hasUnsavedChanges = originalTask != null && (
            currentTitle != originalTask!!.title ||
            currentDescription != originalTask!!.description ||
            currentDueDate != originalTask!!.dueAt?.substringBefore("T")?.takeIf { it.isNotBlank() } ?: "" ||
            currentStatus != originalTask!!.status ||
            currentPriority != originalTask!!.priority
        )
    }

    fun updateTask(context: android.content.Context) {
        viewModelScope.launch {
            _uiState.value = TaskDetailState.Loading

            val task = getCurrentTask() ?: return@launch

            if (!_formState.value.isFormValid) {
                _uiState.value = TaskDetailState.Error(context.getString(R.string.task_fix_form_errors))
                return@launch
            }

            val updated = task.copy(
                title = currentTitle,
                description = currentDescription,
                status = currentStatus,
                priority = currentPriority,
                dueAt = if (currentDueDate.isNotBlank()) "${currentDueDate}T00:00:00Z" else task.dueAt,
                updatedAt = DateTimeUtils.getCurrentTimestamp(),
                serverVersion = (task.serverVersion ?: 0) + 1,
                lastModified = DateTimeUtils.getCurrentTimestamp()
            )

            taskRepository.updateTask(updated)
                .onSuccess {
                    originalTask = updated.copy()
                    hasUnsavedChanges = false
                    _uiState.value = TaskDetailState.Success(updated)
                }
                .onError { e ->
                    _uiState.value = TaskDetailState.Error(
                        context.getString(R.string.task_update_failed, e.message)
                    )
                }
        }
    }

    fun deleteTask(context: android.content.Context) {
        viewModelScope.launch {
            _uiState.value = TaskDetailState.Loading

            val task = getCurrentTask() ?: return@launch

            taskRepository.deleteTask(task)
                .onSuccess {
                    _uiState.value = TaskDetailState.Success(task)
                }
                .onError { e ->
                    _uiState.value = TaskDetailState.Error(
                        context.getString(R.string.task_delete_failed, e.message)
                    )
                }
        }
    }

    fun revertChanges(context: android.content.Context) {
        originalTask?.let { task ->
            currentTitle = task.title
            currentDescription = task.description
            currentDueDate = task.dueAt?.substringBefore("T")?.takeIf { it.isNotBlank() } ?: ""
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
}
