package com.todoapp.ui.notifications

import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.TextView
import androidx.core.content.ContextCompat
import androidx.recyclerview.widget.DiffUtil
import androidx.recyclerview.widget.ListAdapter
import androidx.recyclerview.widget.RecyclerView
import com.todoapp.R
import com.todoapp.data.local.Notification as NotificationEntity
import com.todoapp.utils.DateTimeUtils

class NotificationAdapter(
    private val onMarkAsRead: (Long) -> Unit,
    private val onDelete: (Long) -> Unit
) : ListAdapter<NotificationEntity, NotificationAdapter.NotificationViewHolder>(NotificationDiffCallback()) {

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): NotificationViewHolder {
        val view = LayoutInflater.from(parent.context)
            .inflate(R.layout.item_notification, parent, false)
        return NotificationViewHolder(view)
    }

    override fun onBindViewHolder(holder: NotificationViewHolder, position: Int) {
        holder.bind(getItem(position))
    }

    inner class NotificationViewHolder(itemView: View) : RecyclerView.ViewHolder(itemView) {
        private val titleTextView: TextView = itemView.findViewById(R.id.notificationTitle)
        private val contentTextView: TextView = itemView.findViewById(R.id.notificationContent)
        private val timeTextView: TextView = itemView.findViewById(R.id.notificationTime)
        private val priorityIndicator: View = itemView.findViewById(R.id.priorityIndicator)
        private val unreadBadge: View = itemView.findViewById(R.id.unreadBadge)
        private val markReadButton: View = itemView.findViewById(R.id.btnMarkRead)
        private val deleteButton: View = itemView.findViewById(R.id.btnDelete)

        fun bind(notification: NotificationEntity) {
            titleTextView.text = notification.title
            contentTextView.text = notification.content
            timeTextView.text = DateTimeUtils.formatForDisplay(notification.createdAt ?: "")

            val priorityColorRes = when (notification.priority) {
                "urgent" -> R.color.notification_priority_urgent
                "high" -> R.color.notification_priority_high
                "normal" -> R.color.notification_priority_normal
                "low" -> R.color.notification_priority_low
                else -> R.color.notification_priority_default
            }
            priorityIndicator.setBackgroundColor(
                ContextCompat.getColor(itemView.context, priorityColorRes)
            )

            val isRead = notification.isRead
            unreadBadge.visibility = if (isRead) View.GONE else View.VISIBLE
            markReadButton.visibility = if (isRead) View.GONE else View.VISIBLE

            itemView.setOnClickListener {
                if (!isRead) {
                    onMarkAsRead(notification.id)
                }
            }

            markReadButton.setOnClickListener {
                if (!isRead) {
                    onMarkAsRead(notification.id)
                }
            }

            deleteButton.setOnClickListener {
                onDelete(notification.id)
            }
        }
    }

    class NotificationDiffCallback : DiffUtil.ItemCallback<NotificationEntity>() {
        override fun areItemsTheSame(oldItem: NotificationEntity, newItem: NotificationEntity): Boolean {
            return oldItem.id == newItem.id
        }

        override fun areContentsTheSame(oldItem: NotificationEntity, newItem: NotificationEntity): Boolean {
            return oldItem == newItem
        }
    }
}
