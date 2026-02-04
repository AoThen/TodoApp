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
import com.todoapp.databinding.FragmentAddTaskBinding
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import kotlinx.coroutines.launch
import java.util.Calendar
import java.util.Locale

class AddTaskFragment : Fragment(), DatePickerDialog.OnDateSetListener {

    private var _binding: FragmentAddTaskBinding? = null
    private val binding get() = _binding!!

    private val viewModel: AddTaskViewModel by viewModels()

    private var hasUnsavedChanges: Boolean = false

    override fun onCreateView(
        inflater: LayoutInflater,
        container: ViewGroup?,
        savedInstanceState: Bundle?
    ): View {
        _binding = FragmentAddTaskBinding.inflate(inflater, container, false)
        return binding.root
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)

        setupObservers()
        setupClickListeners()
        setupFormValidation()
    }

    private fun setupObservers() {
        viewLifecycleOwner.lifecycleScope.launch {
            viewLifecycleOwner.repeatOnLifecycle(Lifecycle.State.STARTED) {
                viewModel.uiState.collect { state ->
                    when (state) {
                        is AddTaskState.Idle -> hideLoading()
                        is AddTaskState.Loading -> showLoading()
                        is AddTaskState.Success -> {
                            hideLoading()
                            Toast.makeText(
                                requireContext(),
                                R.string.task_create_success,
                                Toast.LENGTH_SHORT
                            ).show()
                            navigateBack()
                        }
                        is AddTaskState.Error -> {
                            hideLoading()
                            showError(state.message)
                        }
                    }
                }
            }
        }

        viewLifecycleOwner.lifecycleScope.launch {
            viewLifecycleOwner.repeatOnLifecycle(Lifecycle.State.STARTED) {
                viewModel.formState.collect { state ->
                    binding.tilTitle.error = state.titleError
                    binding.tilDescription.error = state.descriptionError
                    binding.btnCreate.isEnabled = state.isFormValid
                }
            }
        }
    }

    private fun setupClickListeners() {
        binding.toolbar.setNavigationOnClickListener {
            if (hasUnsavedChanges) {
                showDiscardConfirmation()
            } else {
                navigateBack()
            }
        }

        binding.btnCancel.setOnClickListener {
            if (hasUnsavedChanges) {
                showDiscardConfirmation()
            } else {
                navigateBack()
            }
        }

        binding.btnCreate.setOnClickListener {
            createTask()
        }

        binding.tilDueDate.setOnClickListener {
            showDatePicker()
        }

        binding.etDueDate.setOnClickListener {
            showDatePicker()
        }

        binding.chipStatusTodo.setOnClickListener {
            selectStatus("todo")
        }
        binding.chipStatusInProgress.setOnClickListener {
            selectStatus("in_progress")
        }
        binding.chipStatusDone.setOnClickListener {
            selectStatus("done")
        }

        binding.chipPriorityLow.setOnClickListener {
            selectPriority("low")
        }
        binding.chipPriorityMedium.setOnClickListener {
            selectPriority("medium")
        }
        binding.chipPriorityHigh.setOnClickListener {
            selectPriority("high")
        }
    }

    private fun setupFormValidation() {
        binding.etTitle.addTextChangedListener(object : TextWatcher {
            override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) {}
            override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) {}
            override fun afterTextChanged(s: Editable?) {
                val title = s.toString().trim()
                viewModel.validateTitle(title, requireContext())
                hasUnsavedChanges = true
            }
        })

        binding.etTitle.setOnFocusChangeListener { _, hasFocus ->
            if (!hasFocus) {
                val title = binding.etTitle.text.toString().trim()
                viewModel.validateTitle(title, requireContext())
            }
        }

        binding.etDescription.addTextChangedListener(object : TextWatcher {
            override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) {}
            override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) {}
            override fun afterTextChanged(s: Editable?) {
                val description = s.toString().trim()
                viewModel.validateDescription(description, requireContext())
                hasUnsavedChanges = true
            }
        })

        binding.etDescription.setOnFocusChangeListener { _, hasFocus ->
            if (!hasFocus) {
                val description = binding.etDescription.text.toString().trim()
                viewModel.validateDescription(description, requireContext())
            }
        }
    }

    private fun selectStatus(status: String) {
        viewModel.updateStatus(status)
        when (status) {
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
        }
    }

    private fun selectPriority(priority: String) {
        viewModel.updatePriority(priority)
        when (priority) {
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
        val date = String.format(Locale.US, "%04d-%02d-%02d", year, month + 1, day)
        binding.etDueDate.setText(date)
        viewModel.validateDueDate(date)
        hasUnsavedChanges = true
    }

    private fun createTask() {
        viewModel.createTask(requireContext())
    }

    private fun showLoading() {
        binding.progressBar.visibility = View.VISIBLE
        binding.tilTitle.isEnabled = false
        binding.tilDescription.isEnabled = false
        binding.chipGroupStatus.isEnabled = false
        binding.chipGroupPriority.isEnabled = false
        binding.tilDueDate.isEnabled = false
        binding.btnCancel.isEnabled = false
        binding.btnCreate.isEnabled = false
    }

    private fun hideLoading() {
        binding.progressBar.visibility = View.GONE
        binding.tilTitle.isEnabled = true
        binding.tilDescription.isEnabled = true
        binding.chipGroupStatus.isEnabled = true
        binding.chipGroupPriority.isEnabled = true
        binding.tilDueDate.isEnabled = true
        binding.btnCancel.isEnabled = true
    }

    private fun showError(message: String) {
        Toast.makeText(requireContext(), message, Toast.LENGTH_LONG).show()
    }

    private fun showDiscardConfirmation() {
        MaterialAlertDialogBuilder(requireContext())
            .setTitle(getString(R.string.discard_changes))
            .setMessage(getString(R.string.discard_changes_message))
            .setPositiveButton(getString(R.string.discard_and_return)) { _, _ ->
                navigateBack()
            }
            .setNegativeButton(getString(R.string.continue_editing), null)
            .show()
    }

    private fun navigateBack() {
        findNavController().navigateUp()
    }

    override fun onDestroyView() {
        super.onDestroyView()
        _binding = null
    }

    companion object {
        private const val TAG = "AddTaskFragment"
    }
}
