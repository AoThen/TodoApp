package com.todoapp.ui.notifications

import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.Button
import android.widget.Toast
import androidx.appcompat.app.AlertDialog
import androidx.fragment.app.Fragment
import androidx.lifecycle.lifecycleScope
import androidx.recyclerview.widget.LinearLayoutManager
import androidx.recyclerview.widget.RecyclerView
import com.todoapp.R
import com.todoapp.data.notify.NotificationManager
import com.todoapp.data.local.Notification as NotificationEntity
import kotlinx.coroutines.*

class NotificationFragment : Fragment() {
    private lateinit var recyclerView: RecyclerView
    private lateinit var adapter: NotificationAdapter
    private lateinit var notificationManager: NotificationManager
    private var userId: Int = 0
    private var currentPage: Int = 1
    private val pageSize: Int = 20
    private var totalNotifications: Int = 0

    private val companion = object {
        const val TAG = "NotificationFragment"
        const val ARG_USER_ID = "user_id"
    }

    override fun onCreateView(
        inflater: LayoutInflater,
        container: ViewGroup?,
        savedInstanceState: Bundle?
    ): View? {
        return inflater.inflate(R.layout.activity_notification, container, false)
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)

        userId = arguments?.getInt(ARG_USER_ID) ?: 0
        notificationManager = NotificationManager(requireContext(), userId)

        setupRecyclerView(view)
        setupButtons(view)
        loadNotifications()
    }

    private fun setupRecyclerView(view: View) {
        recyclerView = view.findViewById(R.id.notificationRecyclerView)
        adapter = NotificationAdapter(
            onMarkAsRead = { notificationId -> handleMarkAsRead(notificationId) },
            onDelete = { notificationId -> handleDelete(notificationId) }
        )

        recyclerView.apply {
            layoutManager = LinearLayoutManager(requireContext())
            adapter = this@NotificationFragment.adapter
        }
    }

    private fun setupButtons(view: View) {
        view.findViewById<Button>(R.id.btnMarkAllRead)?.setOnClickListener {
            lifecycleScope.launch {
                markAllAsRead()
            }
        }

        view.findViewById<Button>(R.id.btnClearAll)?.setOnClickListener {
            showClearConfirmDialog()
        }
    }

    private fun loadNotifications() {
        lifecycleScope.launch {
            try {
                val notifications = notificationManager.getNotifications(currentPage, pageSize)
                adapter.submitList(notifications)
                updateUnreadCount()
            } catch (e: Exception) {
                android.util.Log.e(companion.TAG, "Failed to load notifications", e)
                Toast.makeText(requireContext(), "加载失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private suspend fun updateUnreadCount() {
        val unreadCount = notificationManager.getUnreadCount()
        // 更新UI显示未读数量
        android.util.Log.d(companion.TAG, "Unread count: $unreadCount")
    }

    private fun handleMarkAsRead(notificationId: Long) {
        lifecycleScope.launch {
            val success = notificationManager.markAsRead(notificationId)
            if (success) {
                loadNotifications()
            } else {
                Toast.makeText(requireContext(), "操作失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun handleDelete(notificationId: Long) {
        lifecycleScope.launch {
            val success = notificationManager.deleteNotification(notificationId)
            if (success) {
                loadNotifications()
            } else {
                Toast.makeText(requireContext(), "删除失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private suspend fun markAllAsRead() {
        try {
            val count = notificationManager.markAllAsRead()
            if (count > 0) {
                loadNotifications()
                Toast.makeText(requireContext(), "已标记 $count 条通知为已读", Toast.LENGTH_SHORT).show()
            } else {
                Toast.makeText(requireContext(), "没有未读通知", Toast.LENGTH_SHORT).show()
            }
        } catch (e: Exception) {
            android.util.Log.e(companion.TAG, "Failed to mark all as read", e)
            Toast.makeText(requireContext(), "操作失败", Toast.LENGTH_SHORT).show()
        }
    }

    private fun showClearConfirmDialog() {
        AlertDialog.Builder(requireContext())
            .setTitle("确认清空")
            .setMessage("确认清空所有通知吗？")
            .setPositiveButton("清空") { _, _ ->
                lifecycleScope.launch {
                    clearAllNotifications()
                }
            }
            .setNegativeButton("取消", null)
            .show()
    }

    private suspend fun clearAllNotifications() {
        try {
            val count = notificationManager.clearNotifications(0)
            if (count > 0) {
                loadNotifications()
                Toast.makeText(requireContext(), "已清空 $count 条通知", Toast.LENGTH_SHORT).show()
            } else {
                Toast.makeText(requireContext(), "没有通知可清空", Toast.LENGTH_SHORT).show()
            }
        } catch (e: Exception) {
            android.util.Log.e(companion.TAG, "Failed to clear notifications", e)
            Toast.makeText(requireContext(), "操作失败", Toast.LENGTH_SHORT).show()
        }
    }

    override fun onDestroyView() {
        super.onDestroyView()
        // 不清理notificationManager，因为它是单例
    }
}
