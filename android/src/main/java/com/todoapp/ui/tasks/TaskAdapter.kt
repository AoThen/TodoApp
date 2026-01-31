package com.todoapp.ui.tasks

import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import androidx.core.content.ContextCompat
import androidx.recyclerview.widget.DiffUtil
import androidx.recyclerview.widget.ListAdapter
import androidx.recyclerview.widget.RecyclerView
import com.todoapp.R
import com.todoapp.data.local.Task
import com.todoapp.databinding.ItemTaskBinding

class TaskAdapter(
    private val onTaskClick: (Task) -> Unit,
    private val onTaskComplete: (Task) -> Unit,
    private val onTaskDelete: (Task) -> Unit
) : ListAdapter<Task, TaskAdapter.TaskViewHolder>(TaskDiffCallback()) {

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): TaskViewHolder {
        val binding = ItemTaskBinding.inflate(
            LayoutInflater.from(parent.context),
            parent,
            false
        )
        return TaskViewHolder(binding, onTaskClick, onTaskComplete, onTaskDelete)
    }

    override fun onBindViewHolder(holder: TaskViewHolder, position: Int) {
        holder.bind(getItem(position))
    }

    class TaskViewHolder(
        private val binding: ItemTaskBinding,
        private val onTaskClick: (Task) -> Unit,
        private val onTaskComplete: (Task) -> Unit,
        private val onTaskDelete: (Task) -> Unit
    ) : RecyclerView.ViewHolder(binding.root) {
        
        fun bind(task: Task) {
            binding.apply {
                // Task title and description
                tvTaskTitle.text = task.title
                tvTaskDescription.text = task.description
                
                if (task.description.isBlank()) {
                    tvTaskDescription.visibility = View.GONE
                } else {
                    tvTaskDescription.visibility = View.VISIBLE
                }
                
                // Task status
                val statusText = when (task.status) {
                    "todo" -> root.context.getString(R.string.task_status_todo)
                    "in_progress" -> root.context.getString(R.string.task_status_in_progress)
                    "completed" -> root.context.getString(R.string.task_status_completed)
                    else -> root.context.getString(R.string.task_status_todo)
                }
                tvTaskStatus.text = statusText
                
                // Priority
                val priorityText = when (task.priority) {
                    "low" -> root.context.getString(R.string.task_priority_low)
                    "medium" -> root.context.getString(R.string.task_priority_medium)
                    "high" -> root.context.getString(R.string.task_priority_high)
                    else -> root.context.getString(R.string.task_priority_medium)
                }
                tvTaskPriority.text = priorityText
                
                // Priority color
                val priorityColor = when (task.priority) {
                    "low" -> R.color.priority_low
                    "medium" -> R.color.priority_medium
                    "high" -> R.color.priority_high
                    else -> R.color.priority_medium
                }
                tvTaskPriority.setTextColor(root.context.getColor(priorityColor))
                
                // Completion state
                val isCompleted = task.status == "completed"
                checkboxTask.isChecked = isCompleted
                
                // Visual styling for completed tasks
                if (isCompleted) {
                    tvTaskTitle.alpha = 0.6f
                    tvTaskDescription.alpha = 0.6f
                    tvTaskStatus.alpha = 0.6f
                    tvTaskPriority.alpha = 0.6f
                    cardTask.strokeColor = ContextCompat.getColor(root.context, R.color.task_status_completed)
                } else {
                    tvTaskTitle.alpha = 1.0f
                    tvTaskDescription.alpha = 1.0f
                    tvTaskStatus.alpha = 1.0f
                    tvTaskPriority.alpha = 1.0f
                    cardTask.strokeColor = ContextCompat.getColor(root.context, R.color.md_theme_light_outline)
                }
                
                // Click listeners
                cardTask.setOnClickListener {
                    onTaskClick(task)
                }
                
                checkboxTask.setOnClickListener {
                    onTaskComplete(task)
                }
                
                btnDelete.setOnClickListener {
                    onTaskDelete(task)
                }
            }
        }
    }

    class TaskDiffCallback : DiffUtil.ItemCallback<Task>() {
        override fun areItemsTheSame(oldItem: Task, newItem: Task): Boolean {
            return oldItem.localId == newItem.localId
        }

        override fun areContentsTheSame(oldItem: Task, newItem: Task): Boolean {
            return oldItem == newItem
        }
    }
}