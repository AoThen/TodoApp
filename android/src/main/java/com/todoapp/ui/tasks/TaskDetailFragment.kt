package com.todoapp.ui.tasks

import android.app.DatePickerDialog
import android.os.Bundle
import android.text.Editable
import android.text.TextWatcher
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.DatePicker
import android.widget.Toast
import androidx.fragment.app.Fragment
import androidx.fragment.app.viewModels
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import androidx.navigation.fragment.findNavController
import com.todoapp.R
import com.todoapp.TodoApp
import com.todoapp.data.local.Task
import com.todoapp.utils.DateTimeUtils
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import kotlinx.coroutines.launch
import java.text.SimpleDateFormat
import java.util.Calendar

class TaskDetailFragment : Fragment(), DatePickerDialog.OnDateSetListener {

    private var _binding: com.todoapp.databinding.FragmentTaskDetailBinding? = null
    private val binding get() = _binding!!

    private val taskId: String? by lazy { arguments?.getString("taskId") }

    private val viewModel: TaskDetailViewModel by viewModels()

    private val dateFormat = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss", Locale.getDefault())

    override fun onCreateView(
        inflater: LayoutInflater,
        container: ViewGroup?,
        savedInstanceState: Bundle?
    ): View {
        _binding = com.todoapp.databinding.FragmentTaskDetailBinding.inflate(inflater, container, false)
        return binding.root
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)

        setupObservers()
        setupClickListeners()
        loadData()
    }

    private fun setupObservers() {
        viewLifecycleOwner.lifecycleScope.launch {
            viewLifecycleOwner.repeatOnLifecycle(Lifecycle.State.STARTED) {
                viewModel.uiState.collect { state ->
                    when (state) {
                        is TaskDetailState.Idle -> hideLoading()
                        is TaskDetailState.Loading -> showLoading()
                        is TaskDetailState.Success -> showTaskDetail(state.task)
                        is TaskDetailState.Error -> showError(state.message)
                    }
                }
            }
        }

        viewLifecycleOwner.lifecycleScope.launch {
            viewLifecycleOwner.repeatOnLifecycle(Lifecycle.State.STARTED) {
                viewModel.formState.collect { state ->
                    binding.tilTitle.error = state.titleError
                    binding.btnSave.isEnabled = state.isFormValid
                }
            }
        }
    }

    private fun setupClickListeners() {
        binding.fabEdit.setOnClickListener {
            toggleEditMode()
        }

        binding.btnDelete.setOnClickListener {
            showDeleteConfirmation()
        }

        binding.btnCancel.setOnClickListener {
            if (viewModel.hasUnsavedChanges) {
                showDiscardConfirmation()
            } else {
                navigateBack()
            }
        }

        binding.btnSave.setOnClickListener {
            saveTask()
        }

        binding.tilDueDate.setOnClickListener {
            showDatePicker()
        }

        binding.chipEditStatusTodo.setOnClickListener {
            selectStatus("todo")
        }
        binding.chipEditStatusInProgress.setOnClickListener {
            selectStatus("in_progress")
        }
        binding.chipEditStatusDone.setOnClickListener {
            selectStatus("done")
        }

        binding.chipEditPriorityLow.setOnClickListener {
            selectPriority("low")
        }
        binding.chipPriorityMedium.setOnClickListener {
            selectPriority("medium")
        }
        binding.chipPriorityHigh.setOnClickListener {
            selectPriority("high")
        }

        setupFormValidation()
    }

    private fun setupFormValidation() {
        binding.etTitle.addTextChangedListener(object : TextWatcher {
            override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) {}
            override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) {}
            override fun afterTextChanged(s: Editable?) {
                val title = s.toString().trim()
                viewModel.validateTitle(title, requireContext())
            }
        })

        binding.etTitle.setOnFocusChangeListener { _, hasFocus ->
            if (!hasFocus) {
                val title = binding.etTitle.text.toString().trim()
                viewModel.validateTitle(title, requireContext())
            }
        }

        binding.etEditDescription.addTextChangedListener(object : TextWatcher {
            override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) {}
            override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) {}
            override fun afterTextChanged(s: Editable?) {
                val description = s.toString().trim()
                viewModel.validateDescription(description, requireContext())
            }
        })

        binding.etEditDescription.setOnFocusChangeListener { _, hasFocus ->
            if (!hasFocus) {
                val description = binding.etEditDescription.text.toString().trim()
                viewModel.validateDescription(description, requireContext())
            }
        }

        binding.etEditDueDate.addTextChangedListener(object : TextWatcher {
            override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) {}
            override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) {}
            override fun afterTextChanged(s: Editable?) {
                val dueDate = s.toString().trim()
                viewModel.validateDueDate(dueDate)
            }
        })
    }

    private fun selectStatus(status: String) {
        viewModel.updateStatus(status)
        when (status) {
            "todo" -> {
                binding.chipEditStatusTodo.isChecked = true
                binding.chipEditStatusInProgress.isChecked = false
                binding.chipEditStatusDone.isChecked = false
            }
            "in_progress" -> {
                binding.chipEditStatusTodo.isChecked = false
                binding.chipEditStatusInProgress.isChecked = true
                binding.chipEditStatusDone.isChecked = false
            }
            "done" -> {
                binding.chipEditStatusTodo.isChecked = false
                binding.chipEditStatusInProgress.isChecked = false
                binding.chipEditStatusDone.isChecked = true
            }
        }
    }

    private fun selectPriority(priority: String) {
        viewModel.updatePriority(priority)
        when (priority) {
            "low" -> {
                binding.chipEditPriorityLow.isChecked = true
                binding.chipPriorityMedium.isChecked = false
                binding.chipPriorityHigh.isChecked = false
            }
            "medium" -> {
                binding.chipEditPriorityLow.isChecked = false
                binding.chipPriorityMedium.isChecked = true
                binding.chipPriorityHigh.isChecked = false
            }
            "high" -> {
                binding.chipEditPriorityLow.isChecked = false
                binding.chipPriorityMedium.isChecked = false
                binding.chipPriorityHigh.isChecked = true
            }
        }
    }

    private fun showDatePicker() {
        val calendar = Calendar.getInstance()
        val year = calendar.get(Calendar.YEAR)
        val month = calendar.get(Calendar.MONTH)
        val day = calendar.get(Calendar.DAY_OF_MONTH)

        DatePickerDialog(
            requireContext(),
            this,
            year,
            month,
            day
        ).show()
    }

    override fun onDateSet(view: DatePicker?, year: Int, month: Int, day: Int) {
        val date = String.format("%04d-%02d-%02d", year, month + 1, day)
        binding.etEditDueDate.setText(date)
        viewModel.validateDueDate(date)
    }

    private fun showLoading() {
        binding.progressBar.visibility = View.VISIBLE
        binding.layoutViewMode.visibility = View.GONE
        binding.layoutEditMode.visibility = View.GONE
        binding.fabEdit.visibility = View.GONE
        binding.btnDelete.visibility = View.GONE
    }

    private fun hideLoading() {
        binding.progressBar.visibility = View.GONE
        binding.layoutViewMode.visibility = View.VISIBLE
        binding.layoutEditMode.visibility = View.GONE
        binding.fabEdit.visibility = View.VISIBLE
        binding.btnDelete.visibility = View.VISIBLE
    }

    private fun showTaskDetail(task: Task) {
        binding.layoutViewMode.visibility = View.VISIBLE
        binding.layoutEditMode.visibility = View.GONE

        binding.tvTitle.text = task.title
        binding.tvDescription.text = if (task.description.isBlank()) {
            getString(R.string.task_no_description)
        } else {
            task.description
        }

        when (task.status) {
            "todo" -> {
                binding.chipStatusTodo.isChecked = true
                binding.chipStatusInProgress.isChecked = false
                binding.chipStatusDone.isChecked = false
            }
            "in_progress" -> {
                binding.chipStatusTodo.isChecked = false
                binding.chipStatusInProgress.isChecked = true
                binding.chipStatusDone.isChecked = false
            }
            "done" -> {
                binding.chipStatusTodo.isChecked = false
                binding.chipStatusInProgress.isChecked = false
                binding.chipStatusDone.isChecked = true
            }
            else -> {
                binding.chipStatusTodo.isChecked = true
                binding.chipStatusInProgress.isChecked = false
                binding.chipStatusDone.isChecked = false
            }
        }

        when (task.priority) {
            "low" -> {
                binding.chipPriorityLow.isChecked = true
                binding.chipPriorityMedium.isChecked = false
                binding.chipPriorityHigh.isChecked = false
            }
            "medium" -> {
                binding.chipPriorityLow.isChecked = false
                binding.chipPriorityMedium.isChecked = true
                binding.chipPriorityHigh.isChecked = false
            }
            "high" -> {
                binding.chipPriorityLow.isChecked = false
                binding.chipPriorityMedium.isChecked = false
                binding.chipPriorityHigh.isChecked = true
            }
            else -> {
                binding.chipPriorityLow.isChecked = false
                binding.chipPriorityMedium.isChecked = true
                binding.chipPriorityHigh.isChecked = false
            }
        }

        binding.etDueDate.setText(task.dueAt?.takeIf { it.isNotBlank() }?.substringBefore("T") ?: "")

        val createdAt = try {
            DateTimeUtils.formatForFullDisplay(task.createdAt)
        } catch (e: Exception) {
            getString(R.string.time_unknown)
        }
        binding.tvMetadata.text = getString(R.string.metadata_created_at, createdAt)

        val updatedAt = try {
            DateTimeUtils.formatForFullDisplay(task.updatedAt)
        } catch (e: Exception) {
            getString(R.string.time_unknown)
        }
        binding.tvUpdatedAt.text = getString(R.string.metadata_updated_at, updatedAt)

        binding.priorityIndicator.setBackgroundColor(
            when (task.priority) {
                "low" -> R.color.priority_low
                "high" -> R.color.priority_high
                else -> R.color.priority_medium
            }
        )
    }

    private var _isEditMode = false

    private fun toggleEditMode() {
        viewModel.getCurrentTask()?.let { task ->
            _isEditMode = !_isEditMode
            setEditMode(_isEditMode)

            if (_isEditMode) {
                binding.layoutViewMode.visibility = View.GONE
                binding.layoutEditMode.visibility = View.VISIBLE
                binding.fabEdit.visibility = View.GONE
                binding.btnDelete.visibility = View.GONE

                populateFormFields(task)
            } else {
                binding.layoutViewMode.visibility = View.VISIBLE
                binding.layoutEditMode.visibility = View.GONE
                binding.fabEdit.visibility = View.VISIBLE
                binding.btnDelete.visibility = View.VISIBLE
            }
        }
    }

    private fun setEditMode(isEdit: Boolean) {
        if (isEdit) {
            binding.toolbar.title = getString(R.string.edit_task_title)
            binding.toolbar.subtitle = getString(R.string.metadata_modify_task)
        } else {
            binding.toolbar.title = getString(R.string.task_detail_title)
            binding.toolbar.subtitle = null
        }
    }

    private fun populateFormFields(task: Task) {
        binding.etTitle.setText(task.title)
        binding.etEditDescription.setText(task.description)
        binding.etEditDueDate.setText(task.dueAt?.takeIf { it.isNotBlank() }?.substringBefore("T") ?: "")

        when (task.status) {
            "todo" -> selectStatus("todo")
            "in_progress" -> selectStatus("in_progress")
            "done" -> selectStatus("done")
            else -> selectStatus("todo")
        }

        when (task.priority) {
            "low" -> selectPriority("low")
            "medium" -> selectPriority("medium")
            "high" -> selectPriority("high")
            else -> selectPriority("medium")
        }
    }

    private fun loadData() {
        taskId?.let { viewModel.loadTask(it, requireContext()) }
    }

    private fun saveTask() {
        viewModel.updateTask(requireContext())
    }

    private fun showDeleteConfirmation() {
        viewModel.getCurrentTask()?.let { task ->
            MaterialAlertDialogBuilder(requireContext())
                .setTitle(getString(R.string.confirm_delete))
                .setMessage(getString(R.string.confirm_delete_task_message, task.title))
                .setPositiveButton(getString(R.string.delete)) { _, _ ->
                    viewModel.deleteTask(requireContext())
                    navigateBack()
                }
                .setNegativeButton(getString(R.string.cancel), null)
                .show()
        }
    }

    private fun showDiscardConfirmation() {
        MaterialAlertDialogBuilder(requireContext())
            .setTitle(getString(R.string.discard_changes))
            .setMessage(getString(R.string.discard_changes_message))
            .setPositiveButton(getString(R.string.discard_and_return)) { _, _ ->
                viewModel.revertChanges(requireContext())
                _isEditMode = false
                setEditMode(false)
                binding.layoutViewMode.visibility = View.VISIBLE
                binding.layoutEditMode.visibility = View.GONE
                binding.fabEdit.visibility = View.VISIBLE
                binding.btnDelete.visibility = View.VISIBLE
            }
            .setNegativeButton(getString(R.string.continue_editing), null)
            .show()
    }

    private fun navigateBack() {
        findNavController().navigateUp()
    }

    private fun showError(message: String) {
        Toast.makeText(requireContext(), message, Toast.LENGTH_LONG).show()
    }

    override fun onDestroyView() {
        super.onDestroyView()
        _binding = null
    }

    companion object {
        private const val TAG = "TaskDetailFragment"
    }
}
