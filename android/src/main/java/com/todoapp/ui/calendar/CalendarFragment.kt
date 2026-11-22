package com.todoapp.ui.calendar

import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import androidx.fragment.app.Fragment
import androidx.fragment.app.viewModels
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import androidx.recyclerview.widget.GridLayoutManager
import androidx.recyclerview.widget.LinearLayoutManager
import com.todoapp.R
import com.todoapp.databinding.FragmentCalendarBinding
import com.todoapp.ui.tasks.TaskAdapter
import com.todoapp.ui.tasks.TaskEditBottomSheet
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.launch

@AndroidEntryPoint
class CalendarFragment : Fragment() {

    private var _binding: FragmentCalendarBinding? = null
    private val binding get() = _binding!!

    private val viewModel: CalendarViewModel by viewModels()
    private lateinit var dayAdapter: CalendarDayAdapter
    private lateinit var taskAdapter: TaskAdapter

    override fun onCreateView(
        inflater: LayoutInflater,
        container: ViewGroup?,
        savedInstanceState: Bundle?
    ): View {
        _binding = FragmentCalendarBinding.inflate(inflater, container, false)
        return binding.root
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)

        setupRecyclerViews()
        setupClickListeners()
        setupObservers()
    }

    private fun setupRecyclerViews() {
        dayAdapter = CalendarDayAdapter { day ->
            viewModel.selectDate(day.date)
        }
        binding.rvCalendarDays.apply {
            layoutManager = GridLayoutManager(requireContext(), 7)
            adapter = dayAdapter
        }

        taskAdapter = TaskAdapter(
            onTaskClick = { task ->
                // 与任务列表一致：点击卡片打开编辑底部弹层
                val sheet = TaskEditBottomSheet().apply {
                    arguments = Bundle().apply {
                        putString(TaskEditBottomSheet.ARG_TASK_ID, task.localId)
                    }
                }
                sheet.show(childFragmentManager, TaskEditBottomSheet.TAG)
            },
            onTaskComplete = { task ->
                viewModel.toggleTaskCompletion(task)
            },
            onTaskDelete = { task ->
                viewModel.deleteTask(task)
            }
        )
        binding.rvDayTasks.apply {
            layoutManager = LinearLayoutManager(requireContext())
            adapter = taskAdapter
        }
    }

    private fun setupClickListeners() {
        binding.btnPrevMonth.setOnClickListener { viewModel.previousMonth() }
        binding.btnNextMonth.setOnClickListener { viewModel.nextMonth() }
    }

    private fun setupObservers() {
        viewLifecycleOwner.lifecycleScope.launch {
            viewLifecycleOwner.repeatOnLifecycle(Lifecycle.State.STARTED) {
                viewModel.uiState.collect { state ->
                    binding.tvMonthLabel.text = getString(
                        R.string.calendar_month_format,
                        state.yearMonth.year,
                        state.yearMonth.monthValue
                    )
                    dayAdapter.submitList(state.days)
                    binding.tvSelectedDayLabel.text = getString(
                        R.string.calendar_selected_day_format,
                        state.selectedDate.monthValue,
                        state.selectedDate.dayOfMonth,
                        state.dayTasks.size
                    )
                    taskAdapter.submitList(state.dayTasks)
                    val isEmpty = state.dayTasks.isEmpty()
                    binding.tvDayTasksEmpty.visibility = if (isEmpty) View.VISIBLE else View.GONE
                    binding.rvDayTasks.visibility = if (isEmpty) View.GONE else View.VISIBLE
                }
            }
        }
    }

    override fun onDestroyView() {
        super.onDestroyView()
        _binding = null
    }
}
