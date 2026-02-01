package com.todoapp.ui.notifications

import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.TextView
import androidx.recyclerview.widget.RecyclerView
import com.todoapp.R
import com.todoapp.data.local.Notification as NotificationEntity
import java.text.SimpleDateFormat
import java.util.*

class NotificationAdapter(
    private val onMarkAsRead: (Long) -> Unit,
    private val onDelete: (Long) -> Unit
) : RecyclerView.Adapter<NotificationAdapter.NotificationViewHolder>() {

    private var notifications = listOf<NotificationEntity>()
    private val dateFormat = SimpleDateFormat("MM-dd HH:mm", Locale.getDefault())

    inner class NotificationViewHolder(itemView: View) : RecyclerView.ViewHolder(itemView) {
        val titleTextView: TextView = itemView.findViewById(R.id.notificationTitle)
        val contentTextView: TextView = itemView.findViewById(R.id.notificationContent)
        val timeTextView: TextView = itemView.findViewById(R.id.notificationTime)
        val priorityIndicator: View = itemView.findViewById(R.id.priorityIndicator)
        val unreadBadge: View = itemView.findViewById(R.id.unreadBadge)
        val markReadButton: View = itemView.findViewById(R.id.btnMarkRead)
        val deleteButton: View = itemView.findViewById(R.id.btnDelete)
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): NotificationViewHolder {
        val view = LayoutInflater.from(parent.context)
            .inflate(R.layout.item_notification, parent, false)
        return NotificationViewHolder(view)
    }

    override fun onBindViewHolder(holder: NotificationViewHolder, position: Int) {
        val notification = notifications[position]

        holder.titleTextView.text = notification.title
        holder.contentTextView.text = notification.content

        // 格式化时间
        try {
            val createdAt = notification.createdAt?.let { SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.getDefault()).parse(it) }
            holder.timeTextView.text = createdAt?.let { dateFormat.format(it) } ?: ""
        } catch (e: Exception) {
            holder.timeTextView.text = ""
        }

        // 优先级指示器
        val priorityColor = when (notification.priority) {
            "urgent" -> 0xFFFF0000.toInt()  // 红色
            "high" -> 0xFFFF8000.toInt()    // 橙色
            "normal" -> 0xFF0080FF.toInt() // 蓝色
            "low" -> 0xFF00FF00.toInt()    // 绿色
            else -> 0xFFCCCCCC.toInt()     // 灰色
        }
        holder.priorityIndicator.setBackgroundColor(priorityColor)

        // 未读标记
        if (notification.isRead) {
            holder.unreadBadge.visibility = View.GONE
            holder.markReadButton.visibility = View.GONE
        } else {
            holder.unreadBadge.visibility = View.VISIBLE
            holder.markReadButton.visibility = View.VISIBLE
        }

        // 点击事件
        holder.itemView.setOnClickListener {
            if (!notification.isRead) {
                onMarkAsRead(notification.id)
            }
        }

        // 按钮事件
        holder.markReadButton.setOnClickListener {
            if (!notification.isRead) {
                onMarkAsRead(notification.id)
            }
        }

        holder.deleteButton.setOnClickListener {
            onDelete(notification.id)
        }
    }

    override fun getItemCount(): Int = notifications.size

    fun submitList(newNotifications: List<NotificationEntity>) {
        notifications = newNotifications
        notifyDataSetChanged()
    }
}
