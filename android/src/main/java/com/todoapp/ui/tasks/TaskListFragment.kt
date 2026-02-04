package com.todoapp.ui.tasks

import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.Toast
import androidx.core.content.ContextCompat
import androidx.fragment.app.Fragment
import androidx.fragment.app.viewModels
import androidx.lifecycle.lifecycleScope
import androidx.navigation.fragment.findNavController
import androidx.recyclerview.widget.LinearLayoutManager
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import com.todoapp.R
import com.todoapp.data.local.Task
import com.todoapp.databinding.FragmentTaskListBinding
import kotlinx.coroutines.launch

class TaskListFragment : Fragment() {

    private var _binding: FragmentTaskListBinding? = null
    private val binding get() = _binding!!
    
    private val viewModel: TaskListViewModel by viewModels()
    private lateinit var taskAdapter: TaskAdapter

    override fun onCreateView(
        inflater: LayoutInflater,
        container: ViewGroup?,
        savedInstanceState: Bundle?
    ): View {
        _binding = FragmentTaskListBinding.inflate(inflater, container, false)
        return binding.root
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        
        setupRecyclerView()
        setupClickListeners()
        setupObservers()
    }

    private fun setupRecyclerView() {
        taskAdapter = TaskAdapter(
            onTaskClick = { task: Task ->
                navigateToTaskDetail(task.localId)
            },
            onTaskComplete = { task: Task ->
                viewModel.toggleTaskCompletion(task)
            },
            onTaskDelete = { task: Task ->
                showDeleteTaskDialog(task)
            }
        )
        
        binding.recyclerViewTasks.apply {
            adapter = taskAdapter
            layoutManager = LinearLayoutManager(requireContext())
        }
    }

    private fun setupClickListeners() {
        binding.fabAddTask.setOnClickListener {
            navigateToAddTask()
        }
        
        binding.btnSyncNow.setOnClickListener {
            viewModel.triggerManualSync()
        }
    }

    private fun setupObservers() {
        viewLifecycleOwner.lifecycleScope.launch {
            viewModel.tasks.collect { tasks: List<Task> ->
                taskAdapter.submitList(tasks)
                updateEmptyState(tasks.isEmpty())
            }
        }
        
        viewLifecycleOwner.lifecycleScope.launch {
            viewModel.syncState.collect { state ->
                when (state) {
                    is SyncState.Idle -> {
                        binding.progressBarSync.visibility = View.GONE
                        binding.btnSyncNow.isEnabled = true
                    }
                    is SyncState.Syncing -> {
                        binding.progressBarSync.visibility = View.VISIBLE
                        binding.btnSyncNow.isEnabled = false
                    }
                    is SyncState.Success -> {
                        binding.progressBarSync.visibility = View.GONE
                        binding.btnSyncNow.isEnabled = true
                        Toast.makeText(requireContext(), R.string.sync_success, Toast.LENGTH_SHORT).show()
                    }
                    is SyncState.Error -> {
                        binding.progressBarSync.visibility = View.GONE
                        binding.btnSyncNow.isEnabled = true
                        Toast.makeText(requireContext(), state.message, Toast.LENGTH_LONG).show()
                    }
                }
            }
        }
        
        viewLifecycleOwner.lifecycleScope.launch {
            viewModel.connectionState.collect { isConnected: Boolean ->
                updateSyncStatus(isConnected)
            }
        }
    }

    private fun updateEmptyState(isEmpty: Boolean) {
        if (isEmpty) {
            binding.recyclerViewTasks.visibility = View.GONE
            binding.layoutEmptyState.visibility = View.VISIBLE
        } else {
            binding.recyclerViewTasks.visibility = View.VISIBLE
            binding.layoutEmptyState.visibility = View.GONE
        }
    }

    private fun updateSyncStatus(isConnected: Boolean) {
        val statusText = if (isConnected) {
            getString(R.string.sync_status_online)
        } else {
            getString(R.string.sync_status_offline)
        }
        
        binding.tvSyncStatus.text = statusText
        
        val statusColor = if (isConnected) {
            ContextCompat.getColor(requireContext(), R.color.md_theme_light_primary)
        } else {
            ContextCompat.getColor(requireContext(), R.color.md_theme_light_error)
        }
        
        binding.tvSyncStatus.setTextColor(statusColor)
    }

    private fun navigateToTaskDetail(taskId: String) {
        val bundle = Bundle().apply {
            putString("taskId", taskId)
        }
        findNavController().navigate(
            R.id.action_taskListFragment_to_taskDetailFragment,
            bundle
        )
    }

    private fun navigateToAddTask() {
        findNavController().navigate(R.id.action_taskListFragment_to_addTaskFragment)
    }

    private fun showDeleteTaskDialog(task: Task) {
        MaterialAlertDialogBuilder(requireContext())
            .setTitle(getString(R.string.confirm_delete))
            .setMessage(getString(R.string.confirm_delete_task_message, task.title))
            .setPositiveButton(getString(R.string.delete)) { _, _ ->
                viewModel.deleteTask(task)
            }
            .setNegativeButton(getString(R.string.cancel), null)
            .show()
    }

    override fun onDestroyView() {
        super.onDestroyView()
        _binding = null
    }
}