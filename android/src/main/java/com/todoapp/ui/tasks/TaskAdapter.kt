package com.todoapp.ui.tasks

import android.graphics.Paint
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
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

class TaskAdapter(
    private val onTaskClick: (Task) -> Unit,
    private val onTaskComplete: (Task) -> Unit
) : ListAdapter<Task, TaskAdapter.TaskViewHolder>(TaskDiffCallback()) {

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): TaskViewHolder {
        val binding = ItemTaskBinding.inflate(
            LayoutInflater.from(parent.context),
            parent,
            false
        )
        return TaskViewHolder(binding, onTaskClick, onTaskComplete)
    }

    override fun onBindViewHolder(holder: TaskViewHolder, position: Int) {
        holder.bind(getItem(position))
    }

    companion object {
        // 仅主线程 bind 中使用，避免每次绑定重复创建
        private val dateFormat = SimpleDateFormat("yyyy-MM-dd", Locale.US)
    }

    class TaskViewHolder(
        private val binding: ItemTaskBinding,
        private val onTaskClick: (Task) -> Unit,
        private val onTaskComplete: (Task) -> Unit
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

                // 优先级：文字 + 左侧色条同色（原型 c1 红 / c2 黄 / c3 绿）
                val (priorityTextRes, priorityColorRes) = when (task.priority) {
                    "low" -> R.string.task_priority_low to R.color.priority_low
                    "high" -> R.string.task_priority_high to R.color.priority_high
                    else -> R.string.task_priority_medium to R.color.priority_medium
                }
                tvTaskPriority.text = root.context.getString(priorityTextRes)
                val priorityColor = root.context.getColor(priorityColorRes)
                tvTaskPriority.setTextColor(priorityColor)
                priorityBar.setBackgroundColor(priorityColor)

                // 状态 chip：文字与底色对齐原型（待办蓝 / 进行中琥珀 / 已完成绿 / 已归档灰）
                val (chipBgRes, chipTextRes, statusTextRes) = when (task.status) {
                    "todo" -> Triple(R.color.info_container, R.color.info, R.string.task_status_todo)
                    "in_progress" -> Triple(R.color.warning_container, R.color.warning, R.string.task_status_in_progress)
                    "done" -> Triple(R.color.success_container, R.color.success, R.string.task_status_completed)
                    else -> Triple(R.color.muted_container, R.color.text_muted, R.string.task_status_archived)
                }
                tvTaskStatus.text = root.context.getString(statusTextRes)
                tvTaskStatus.chipBackgroundColor =
                    android.content.res.ColorStateList.valueOf(root.context.getColor(chipBgRes))
                tvTaskStatus.setTextColor(root.context.getColor(chipTextRes))

                // 截止日期（今天显示「今天」）
                val dueDate = task.dueAt?.takeIf { it.isNotBlank() }?.substringBefore("T")
                if (dueDate.isNullOrBlank()) {
                    tvDueDate.visibility = View.GONE
                } else {
                    tvDueDate.visibility = View.VISIBLE
                    val today = dateFormat.format(Date())
                    tvDueDate.text = if (dueDate == today) {
                        root.context.getString(R.string.due_today)
                    } else {
                        dueDate
                    }
                }

                // Completion state（原型 .t-card.done：绿色勾选 + 标题删除线置灰）
                val isCompleted = task.status == "done"
                checkboxTask.isChecked = isCompleted
                if (isCompleted) {
                    tvTaskTitle.paintFlags = tvTaskTitle.paintFlags or Paint.STRIKE_THRU_TEXT_FLAG
                    tvTaskTitle.alpha = 0.6f
                    tvTaskDescription.alpha = 0.7f
                    tvTaskStatus.alpha = 0.6f
                    tvTaskPriority.alpha = 0.6f
                    tvDueDate.alpha = 0.6f
                    cardTask.strokeColor = ContextCompat.getColor(root.context, R.color.success)
                } else {
                    tvTaskTitle.paintFlags = tvTaskTitle.paintFlags and Paint.STRIKE_THRU_TEXT_FLAG.inv()
                    tvTaskTitle.alpha = 1.0f
                    tvTaskDescription.alpha = 1.0f
                    tvTaskStatus.alpha = 1.0f
                    tvTaskPriority.alpha = 1.0f
                    tvDueDate.alpha = 1.0f
                    cardTask.strokeColor = ContextCompat.getColor(root.context, R.color.app_border)
                }

                // Click listeners（卡片点击打开编辑弹层；checkbox 切换完成态；删除入口在编辑弹层）
                cardTask.setOnClickListener {
                    onTaskClick(task)
                }

                checkboxTask.setOnClickListener {
                    onTaskComplete(task)
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
