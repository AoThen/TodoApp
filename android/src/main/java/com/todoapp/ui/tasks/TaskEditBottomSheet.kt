package com.todoapp.ui.tasks

import android.app.DatePickerDialog
import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.DatePicker
import android.widget.Toast
import androidx.fragment.app.viewModels
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import com.google.android.material.bottomsheet.BottomSheetDialogFragment
import com.todoapp.R
import com.todoapp.databinding.FragmentTaskEditBottomSheetBinding
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.launch
import java.util.Calendar
import java.util.Locale

/**
 * 任务编辑底部弹层：点任务卡片直接打开，含标题/备注/状态(4态)/优先级/截止日期。
 * 复用 TaskDetailViewModel 的加载与更新逻辑，保存走 offline-first（Room + delta 队列）。
 */
@AndroidEntryPoint
class TaskEditBottomSheet : BottomSheetDialogFragment(), DatePickerDialog.OnDateSetListener {

    private var _binding: FragmentTaskEditBottomSheetBinding? = null
    private val binding get() = _binding!!

    private val taskId: String? by lazy { arguments?.getString(ARG_TASK_ID) }

    private val viewModel: TaskDetailViewModel by viewModels()

    // 首次 Success 为加载完成（回填表单），其后 Success 为保存成功（关闭弹层）
    private var loaded = false

    override fun onCreateView(
        inflater: LayoutInflater,
        container: ViewGroup?,
        savedInstanceState: Bundle?
    ): View {
        _binding = FragmentTaskEditBottomSheetBinding.inflate(inflater, container, false)
        return binding.root
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        setupClickListeners()
        setupObservers()
        taskId?.let { viewModel.loadTask(it, requireContext()) }
    }

    private fun setupClickListeners() {
        binding.btnCancel.setOnClickListener { dismiss() }
        binding.btnSave.setOnClickListener {
            viewModel.updateTask(requireContext())
        }

        binding.tilDueDate.setOnClickListener { showDatePicker() }
        binding.etDueDate.setOnClickListener { showDatePicker() }

        binding.chipStatusTodo.setOnClickListener { selectStatus("todo") }
        binding.chipStatusInProgress.setOnClickListener { selectStatus("in_progress") }
        binding.chipStatusDone.setOnClickListener { selectStatus("done") }
        binding.chipStatusArchived.setOnClickListener { selectStatus("archived") }

        binding.chipPriorityLow.setOnClickListener { selectPriority("low") }
        binding.chipPriorityMedium.setOnClickListener { selectPriority("medium") }
        binding.chipPriorityHigh.setOnClickListener { selectPriority("high") }
    }

    private fun setupObservers() {
        viewLifecycleOwner.lifecycleScope.launch {
            viewLifecycleOwner.repeatOnLifecycle(Lifecycle.State.STARTED) {
                viewModel.formState.collect { state ->
                    binding.tilTitle.error = state.titleError
                    binding.btnSave.isEnabled = state.isFormValid
                }
            }
        }

        viewLifecycleOwner.lifecycleScope.launch {
            viewLifecycleOwner.repeatOnLifecycle(Lifecycle.State.STARTED) {
                viewModel.uiState.collect { state ->
                    when (state) {
                        is TaskDetailState.Loading -> {
                            binding.progressBar.visibility = View.VISIBLE
                        }
                        is TaskDetailState.Error -> {
                            binding.progressBar.visibility = View.GONE
                            Toast.makeText(requireContext(), state.message, Toast.LENGTH_LONG).show()
                        }
                        is TaskDetailState.Success -> {
                            binding.progressBar.visibility = View.GONE
                            if (!loaded) {
                                loaded = true
                                populateForm(state.task)
                            } else {
                                Toast.makeText(
                                    requireContext(),
                                    R.string.task_update_success,
                                    Toast.LENGTH_SHORT
                                ).show()
                                dismiss()
                            }
                        }
                        is TaskDetailState.Idle -> { }
                    }
                }
            }
        }
    }

    private fun populateForm(task: com.todoapp.data.local.Task) {
        binding.etTitle.setText(task.title)
        binding.etDescription.setText(task.description)
        binding.etDueDate.setText(task.dueAt?.takeIf { it.isNotBlank() }?.substringBefore("T") ?: "")
        selectStatus(task.status)
        selectPriority(task.priority)
    }

    private fun selectStatus(status: String) {
        viewModel.updateStatus(status)
        binding.chipStatusTodo.isChecked = status == "todo"
        binding.chipStatusInProgress.isChecked = status == "in_progress"
        binding.chipStatusDone.isChecked = status == "done"
        binding.chipStatusArchived.isChecked = status == "archived"
    }

    private fun selectPriority(priority: String) {
        viewModel.updatePriority(priority)
        binding.chipPriorityLow.isChecked = priority == "low"
        binding.chipPriorityMedium.isChecked = priority == "medium"
        binding.chipPriorityHigh.isChecked = priority == "high"
    }

    private fun showDatePicker() {
        val calendar = Calendar.getInstance()
        val year = calendar.get(Calendar.YEAR)
        val month = calendar.get(Calendar.MONTH)
        val day = calendar.get(Calendar.DAY_OF_MONTH)

        DatePickerDialog(requireContext(), this, year, month, day).show()
    }

    override fun onDateSet(view: DatePicker?, year: Int, month: Int, day: Int) {
        val date = String.format(Locale.US, "%04d-%02d-%02d", year, month + 1, day)
        binding.etDueDate.setText(date)
        viewModel.validateDueDate(date)
    }

    override fun onDestroyView() {
        super.onDestroyView()
        _binding = null
    }

    companion object {
        const val TAG = "TaskEditBottomSheet"
        const val ARG_TASK_ID = "taskId"
    }
}
