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
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.navigation.fragment.findNavController
import androidx.navigation.fragment.navArgs
import com.todoapp.R
import com.todoapp.TodoApp
import com.todoapp.data.Task
import com.todoapp.data.local.AppDatabase
import kotlinx.coroutines.launch
import java.text.SimpleDateFormat
import java.util.Calendar
import java.util.Locale

class TaskDetailFragment : Fragment() {

    private var _binding: com.todoapp.databinding.FragmentTaskDetailBinding? = null
    private val binding get(): com.todoapp.databinding.FragmentTaskDetailBinding = _binding!!

    private val args: com.todoapp.ui.tasks.TaskDetailFragmentArgs by navArgs()

    private lateinit var viewModel: TaskDetailViewModel

    // 日期选择器
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

        val database = (requireActivity().application as TodoApp).database
        val factory = TaskDetailViewModel.Factory(database.taskDao())
        viewModel = ViewModelProvider(this, factory)[TaskDetailViewModel::class.java]

        setupObservers()
        setupClickListeners()
        loadData()
    }

    private fun setupObservers() {
        viewLifecycleOwner.lifecycleScope.launch {
            lifecycleOwner.repeatOnLifecycle(androidx.lifecycle.Lifecycle.State.STARTED) {
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
            lifecycleOwner.repeatOnLifecycle(androidx.lifecycle.Lifecycle.State.STARTED) {
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
            if (viewModel.getCurrentTask()?.isEdited == true) {
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

        // 状态 Chip选择
        binding.chipEditStatusTodo.setOnClickListener {
            selectStatus("todo")
        }
        binding.chipEditStatusInProgress.setOnClickListener {
            selectStatus("in_progress")
        }
        binding.chipEditStatusDone.setOnClickListener {
            selectStatus("done")
        }

        // 优先级 Chip选择
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
                viewModel.validateTitle(title)
            }
        })

        binding.etTitle.setOnFocusChangeListener { _, hasFocus ->
            if (!hasFocus) {
                val title = binding.etTitle.text.toString().trim()
                viewModel.validateTitle(title)
            }
        }

        binding.etEditDescription.addTextChangedListener(object : TextWatcher {
            override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) {}
            override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) {}
            override fun afterTextChanged(s: Editable?) {
                val description = s.toString().trim()
                viewModel.validateDescription(description)
            }
        })

        binding.etEditDescription.setOnFocusChangeListener { _, hasFocus ->
            if (!hasFocus) {
                val description = binding.etEditDescription.text.toString().trim()
                viewModel.validateDescription(description)
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
                binding.chipEditPriorityMedium.isChecked = false
                binding.chipEditPriorityHigh.isChecked = false
            }
            "medium" -> {
                binding.chipEditPriorityLow.isChecked = false
                binding.chipEditPriorityMedium.isChecked = true
                binding.chipEditPriorityHigh.isChecked = false
            }
            "high" -> {
                binding.chipEditPriorityLow.isChecked = false
                binding.chipEditPriorityMedium.isChecked = false
                binding.chipEditPriorityHigh.isChecked = true
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
        binding.tvDescription.text = if (task.description.isBlank()) "无描述" else task.description

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

        binding.etDueDate.setText(if (task.dueAt.isNotBlank()) task.dueAt.substringBefore("T") else "")

        val createdAt = try {
            val parsed = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss", Locale.getDefault()).parse(task.createdAt)
            SimpleDateFormat("yyyy-MM-dd", Locale.getDefault()).format(parsed)
        } catch (e: Exception) {
            "未知"
        }
        binding.tvMetadata.text = "创建时间: $createdAt"

        val updatedAt = try {
            val parsed = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss", Locale.getDefault()).parse(task.updatedAt)
            SimpleDateFormat("yyyy-MM-dd HH:mm", Locale.getDefault()).format(parsed)
        } catch (e: Exception) {
            "未知"
        }
        binding.tvUpdatedAt.text = "更新时间: $updatedAt"

        binding.priorityIndicator.setBackgroundColor(
            when (task.priority) {
                "low" -> R.color.priority_low
                "high" -> R.color.priority_high
                else -> R.color.priority_medium
            }
        )
    }

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

    private var _isEditMode = false

    private fun setEditMode(isEdit: Boolean) {
        if (isEdit) {
            binding.toolbar.title = "编辑任务"
            binding.toolbar.subtitle = "修改任务信息"
        } else {
            binding.toolbar.title = "任务详情"
            binding.toolbar.subtitle = null
        }
    }

    private fun populateFormFields(task: Task) {
        binding.etTitle.setText(task.title)
        binding.etEditDescription.setText(task.description)
        binding.etEditDueDate.setText(if (task.dueAt.isNotBlank()) task.dueAt.substringBefore("T") else "")

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
        viewModel.loadTask(args.taskId)
    }

    private fun saveTask() {
        viewModel.updateTask()
    }

    private fun showDeleteConfirmation() {
        viewModel.getCurrentTask()?.let { task ->
            androidx.appcompat.app.AlertDialog.Builder(requireContext())
                .setTitle("确认删除")
                .setMessage("确定要删除任务 '${task.title}' 吗？此操作无法撤销。")
                .setPositiveButton("删除") { _, _ ->
                    viewModel.deleteTask(task.localId.toString())
                    navigateBack()
                }
                .setNegativeButton("取消", null)
                .show()
        }
    }

    private fun showDiscardConfirmation() {
        androidx.appcompat.app.AlertDialog.Builder(requireContext())
            .setTitle("放弃更改")
            .setMessage("您有未保存的更改，确定要返回吗？")
            .setPositiveButton("放弃返回") { _, _ ->
                viewModel.revertChanges()
                _isEditMode = false
                setEditMode(false)
                binding.layoutViewMode.visibility = View.VISIBLE
                binding.layoutEditMode.visibility = View.GONE
                binding.fabEdit.visibility = View.VISIBLE
                binding.btnDelete.visibility = View.VISIBLE
            }
            .setNegativeButton("继续编辑", null)
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
        private const TAG = "TaskDetailFragment"
    }
}
