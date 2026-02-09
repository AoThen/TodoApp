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

    private object Config {
        val TAG = "NotificationFragment"
        val ARG_USER_ID = "user_id"
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

        userId = arguments?.getInt(Config.ARG_USER_ID) ?: 0
        notificationManager = NotificationManager(requireContext(), userId, lifecycleScope)

        setupRecyclerView(view)
        setupButtons(view)
        loadNotifications()
    }

    override fun onDestroyView() {
        super.onDestroyView()
        notificationManager.cleanup()
    }
}
