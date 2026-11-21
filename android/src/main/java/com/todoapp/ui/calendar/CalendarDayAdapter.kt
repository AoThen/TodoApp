package com.todoapp.ui.calendar

import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import androidx.recyclerview.widget.DiffUtil
import androidx.recyclerview.widget.ListAdapter
import androidx.recyclerview.widget.RecyclerView
import com.todoapp.databinding.ItemCalendarDayBinding

/** 月历网格（7 列 × 6 行）日期格子适配器 */
class CalendarDayAdapter(
    private val onDayClick: (CalendarDay) -> Unit
) : ListAdapter<CalendarDay, CalendarDayAdapter.DayViewHolder>(DiffCallback()) {

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): DayViewHolder {
        val binding = ItemCalendarDayBinding.inflate(
            LayoutInflater.from(parent.context),
            parent,
            false
        )
        return DayViewHolder(binding, onDayClick)
    }

    override fun onBindViewHolder(holder: DayViewHolder, position: Int) {
        holder.bind(getItem(position))
    }

    class DayViewHolder(
        private val binding: ItemCalendarDayBinding,
        private val onDayClick: (CalendarDay) -> Unit
    ) : RecyclerView.ViewHolder(binding.root) {

        fun bind(day: CalendarDay) {
            binding.tvDayNumber.text = day.date.dayOfMonth.toString()

            // 跨月补位格子弱化显示；选中日期以主色圆形高亮
            binding.tvDayNumber.alpha = if (day.inMonth) 1.0f else 0.3f
            if (day.isSelected) {
                binding.tvDayNumber.setBackgroundResource(com.todoapp.R.drawable.calendar_day_selected)
            } else {
                binding.tvDayNumber.background = null
            }

            binding.vTaskDot.visibility = if (day.taskCount > 0) View.VISIBLE else View.GONE
            binding.root.setOnClickListener { onDayClick(day) }
        }
    }

    class DiffCallback : DiffUtil.ItemCallback<CalendarDay>() {
        override fun areItemsTheSame(oldItem: CalendarDay, newItem: CalendarDay): Boolean {
            return oldItem.date == newItem.date
        }

        override fun areContentsTheSame(oldItem: CalendarDay, newItem: CalendarDay): Boolean {
            return oldItem == newItem
        }
    }
}
