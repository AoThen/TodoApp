package com.todoapp.ui.tasks

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.todoapp.R
import com.todoapp.data.local.Task
import com.todoapp.data.repository.TaskRepository
import com.todoapp.utils.DateTimeUtils
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import java.util.UUID
import javax.inject.Inject

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

@HiltViewModel
class AddTaskViewModel @Inject constructor(
    private val taskRepository: TaskRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow<AddTaskState>(AddTaskState.Idle)
    val uiState: StateFlow<AddTaskState> = _uiState.asStateFlow()

    private val _formState = MutableStateFlow(AddTaskFormState())
    val formState: StateFlow<AddTaskFormState> = _formState.asStateFlow()

    private var currentTitle: String = ""
    private var currentDescription: String = ""
    private var currentDueDate: String = ""
    private var currentStatus: String = "todo"
    private var currentPriority: String = "medium"

    fun validateTitle(title: String, context: android.content.Context) {
        currentTitle = title.trim()
        val error = when {
            currentTitle.isEmpty() -> context.getString(R.string.task_title_empty)
            currentTitle.length > 200 -> context.getString(R.string.task_title_too_long)
            else -> null
        }
        _formState.value = _formState.value.copy(titleError = error)
    }

    fun validateDescription(description: String, context: android.content.Context) {
        currentDescription = description.trim()
        val error = when {
            currentDescription.length > 2000 -> context.getString(R.string.task_description_too_long)
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

    fun validateForm(context: android.content.Context) {
        validateTitle(currentTitle, context)
        validateDescription(currentDescription, context)
        validateDueDate(currentDueDate)
    }

    fun createTask(context: android.content.Context) {
        viewModelScope.launch {
            _uiState.value = AddTaskState.Loading

            if (!_formState.value.isFormValid) {
                _uiState.value = AddTaskState.Error(context.getString(R.string.task_fix_form_errors))
                return@launch
            }

            taskRepository.createTask(
                title = currentTitle,
                description = currentDescription,
                status = currentStatus,
                priority = currentPriority,
                dueAt = if (currentDueDate.isNotBlank()) "${currentDueDate}T00:00:00Z" else null
            ).onSuccess { task ->
                _uiState.value = AddTaskState.Success(task)
                resetForm()
            }.onError { e ->
                _uiState.value = AddTaskState.Error(
                    context.getString(R.string.task_create_failed, e.message)
                )
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
}
