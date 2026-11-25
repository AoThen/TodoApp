package com.todoapp.ui.tasks

import android.content.res.ColorStateList
import android.os.Bundle
import android.text.Editable
import android.text.TextWatcher
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.PopupMenu
import android.widget.Toast
import androidx.core.content.ContextCompat
import androidx.fragment.app.Fragment
import androidx.fragment.app.viewModels
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import androidx.navigation.fragment.findNavController
import androidx.recyclerview.widget.ItemTouchHelper
import androidx.recyclerview.widget.LinearLayoutManager
import androidx.recyclerview.widget.RecyclerView
import androidx.work.WorkManager
import com.todoapp.R
import com.todoapp.config.AppConfig
import com.todoapp.data.local.Task
import com.todoapp.data.sync.DeltaSyncWorker
import com.todoapp.databinding.FragmentTaskListBinding
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.launch
import java.util.Locale

@AndroidEntryPoint
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
        setupSearch()
        setupObservers()

        // 进入主页面即确保周期同步已调度（KEEP 策略下重复调用无副作用）；
        // 游客模式无登录态，取消周期同步避免无谓失败，登录后再恢复调度
        if (AppConfig.isGuestMode(requireContext())) {
            WorkManager.getInstance(requireContext()).cancelUniqueWork(DeltaSyncWorker.WORK_NAME)
        } else {
            DeltaSyncWorker.schedule(requireContext())
        }
    }

    private fun setupRecyclerView() {
        taskAdapter = TaskAdapter(
            onTaskClick = { task: Task ->
                // 点卡片直接打开编辑底部弹层（删除入口在弹层内）
                val sheet = TaskEditBottomSheet().apply {
                    arguments = Bundle().apply {
                        putString(TaskEditBottomSheet.ARG_TASK_ID, task.localId)
                    }
                }
                sheet.show(childFragmentManager, TaskEditBottomSheet.TAG)
            },
            onTaskComplete = { task: Task ->
                viewModel.toggleTaskCompletion(task)
            }
        )
        
        binding.recyclerViewTasks.apply {
            adapter = taskAdapter
            layoutManager = LinearLayoutManager(requireContext())
        }
        setupDragAndDrop()
        setupSortMenu()
    }

    /** 手动排序：长按拖拽（仅 manual 模式启用），释放时按相邻 sort_order 中点持久化 */
    private fun setupDragAndDrop() {
        var dragFrom = RecyclerView.NO_POSITION
        val callback = object : ItemTouchHelper.SimpleCallback(
            ItemTouchHelper.UP or ItemTouchHelper.DOWN,
            0
        ) {
            // 注：recyclerview 经传递依赖版本低于 1.2，无 bindingAdapterPosition，
            // 使用等价的 adapterPosition（1.2 起标记过时，仅产生编译警告不影响构建）
            override fun onMove(
                recyclerView: RecyclerView,
                viewHolder: RecyclerView.ViewHolder,
                target: RecyclerView.ViewHolder
            ): Boolean {
                val from = viewHolder.adapterPosition
                val to = target.adapterPosition
                if (from == RecyclerView.NO_POSITION || to == RecyclerView.NO_POSITION) return false
                val current = taskAdapter.currentList.toMutableList()
                if (from !in current.indices || to !in current.indices) return false
                val item = current.removeAt(from)
                current.add(to, item)
                taskAdapter.submitList(current)
                return true
            }

            override fun onSwiped(viewHolder: RecyclerView.ViewHolder, direction: Int) {}

            override fun isLongPressDragEnabled(): Boolean =
                viewModel.sortMode.value == TaskListViewModel.SORT_MODE_MANUAL

            override fun onSelectedChanged(viewHolder: RecyclerView.ViewHolder?, actionState: Int) {
                super.onSelectedChanged(viewHolder, actionState)
                if (actionState == ItemTouchHelper.ACTION_STATE_DRAG && viewHolder != null) {
                    dragFrom = viewHolder.adapterPosition
                }
            }

            override fun clearView(recyclerView: RecyclerView, viewHolder: RecyclerView.ViewHolder) {
                super.clearView(recyclerView, viewHolder)
                val to = viewHolder.adapterPosition
                if (dragFrom != RecyclerView.NO_POSITION && to != RecyclerView.NO_POSITION && dragFrom != to) {
                    // currentList 已是 onMove 提交后的移动结果，被拖任务就在 to 位置，
                    // 直接按其新邻居计算中点，不能再用拖拽前索引重复模拟移动
                    viewModel.persistReorderAt(taskAdapter.currentList, to)
                }
                dragFrom = RecyclerView.NO_POSITION
            }
        }
        ItemTouchHelper(callback).attachToRecyclerView(binding.recyclerViewTasks)
    }

    /** 排序模式入口：弹出菜单切换，manual 模式同时启用长按拖拽 */
    private fun setupSortMenu() {
        binding.btnSortMode.setOnClickListener { anchor ->
            val modes = listOf(
                TaskListViewModel.SORT_MODE_CREATED to R.string.sort_created_at,
                TaskListViewModel.SORT_MODE_MANUAL to R.string.sort_manual,
                TaskListViewModel.SORT_MODE_UPDATED to R.string.sort_updated_at,
                TaskListViewModel.SORT_MODE_PRIORITY to R.string.sort_priority,
            )
            val popup = PopupMenu(requireContext(), anchor)
            modes.forEachIndexed { index, (_, labelRes) ->
                popup.menu.add(0, index, 0, labelRes)
            }
            popup.setOnMenuItemClickListener { item ->
                modes.getOrNull(item.itemId)?.let { (mode, _) ->
                    viewModel.setSortMode(mode)
                }
                true
            }
            popup.show()
        }
    }

    private fun setupClickListeners() {
        binding.fabAddTask.setOnClickListener {
            navigateToAddTask()
        }
        
        binding.btnSyncNow.setOnClickListener {
            if (AppConfig.isGuestMode(requireContext())) {
                Toast.makeText(requireContext(), R.string.guest_sync_disabled, Toast.LENGTH_SHORT).show()
                return@setOnClickListener
            }
            viewModel.triggerManualSync()
        }
    }

    private fun setupObservers() {
        viewLifecycleOwner.lifecycleScope.launch {
            viewLifecycleOwner.repeatOnLifecycle(Lifecycle.State.STARTED) {
                viewModel.tasks.collect { tasks: List<Task> ->
                    taskAdapter.submitList(tasks)
                    updateEmptyState(tasks.isEmpty(), viewModel.searchQuery.value.isNotBlank())
                    updateGreeting(tasks)
                }
            }
        }

        viewLifecycleOwner.lifecycleScope.launch {
            viewLifecycleOwner.repeatOnLifecycle(Lifecycle.State.STARTED) {
                viewModel.syncState.collect { state ->
                    when (state) {
                        is SyncState.Idle -> {
                            binding.progressBarSync.visibility = View.GONE
                            binding.btnSyncNow.isEnabled = true
                            refreshSyncChip()
                        }
                        is SyncState.Syncing -> {
                            binding.progressBarSync.visibility = View.VISIBLE
                            binding.btnSyncNow.isEnabled = false
                            updateSyncChip(
                                getString(R.string.sync_status_syncing),
                                R.color.warning,
                                R.color.warning_container
                            )
                        }
                        is SyncState.Success -> {
                            binding.progressBarSync.visibility = View.GONE
                            binding.btnSyncNow.isEnabled = true
                            refreshSyncChip()
                            Toast.makeText(requireContext(), R.string.sync_success, Toast.LENGTH_SHORT).show()
                        }
                        is SyncState.Error -> {
                            binding.progressBarSync.visibility = View.GONE
                            binding.btnSyncNow.isEnabled = true
                            refreshSyncChip()
                            Toast.makeText(requireContext(), state.message, Toast.LENGTH_LONG).show()
                        }
                    }
                }
            }
        }

        viewLifecycleOwner.lifecycleScope.launch {
            viewLifecycleOwner.repeatOnLifecycle(Lifecycle.State.STARTED) {
                viewModel.connectionState.collect { isConnected: Boolean ->
                    updateSyncStatus(isConnected)
                }
            }
        }
    }

    private fun setupSearch() {
        binding.etSearch.addTextChangedListener(object : TextWatcher {
            override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) {}
            override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) {}
            override fun afterTextChanged(s: Editable?) {
                viewModel.setSearchQuery(s?.toString() ?: "")
            }
        })
    }

    /** 问候区：时段问候 + 登录邮箱前缀 + 未完成任务数（原型 .greet） */
    private fun updateGreeting(tasks: List<Task>) {
        val email = AppConfig.getUserEmail(requireContext())
        val displayName = email.substringBefore("@").ifBlank { getString(R.string.guest_display_name) }
        val hour = java.util.Calendar.getInstance().get(java.util.Calendar.HOUR_OF_DAY)
        val greetingRes = when (hour) {
            in 6..11 -> R.string.greeting_morning
            in 12..17 -> R.string.greeting_afternoon
            else -> R.string.greeting_evening
        }
        binding.tvGreeting.text = getString(greetingRes) + "，" + displayName
        binding.tvGreetingSub.text =
            getString(R.string.greeting_task_summary, tasks.count { it.status != "done" })
        binding.tvAvatar.text = displayName.take(1).uppercase(Locale.ROOT)
    }

    /** 同步 chip：文案 + 文字/底色（原型 .sync-chip：绿=已同步，琥珀=同步中，红=离线） */
    private fun updateSyncChip(text: String, textColorRes: Int, bgColorRes: Int) {
        binding.tvSyncChip.text = text
        binding.tvSyncChip.setTextColor(ContextCompat.getColor(requireContext(), textColorRes))
        binding.tvSyncChip.backgroundTintList =
            ColorStateList.valueOf(ContextCompat.getColor(requireContext(), bgColorRes))
    }

    /** 按当前连接状态复位同步 chip（手动同步结束后调用，避免停留在「同步中」） */
    private fun refreshSyncChip() {
        if (viewModel.connectionState.value) {
            updateSyncChip(
                getString(R.string.sync_synced),
                R.color.success,
                R.color.success_container
            )
        } else {
            updateSyncChip(
                getString(R.string.sync_status_offline),
                R.color.danger,
                R.color.muted_container
            )
        }
    }

    private fun updateEmptyState(isEmpty: Boolean, isSearching: Boolean = false) {
        if (isEmpty) {
            binding.recyclerViewTasks.visibility = View.GONE
            binding.layoutEmptyState.visibility = View.VISIBLE
            // 区分「暂无任务」与「搜索无结果」
            binding.tvEmptyTitle.text =
                getString(if (isSearching) R.string.tasks_no_match else R.string.tasks_empty)
            binding.tvEmptyHint.text =
                getString(if (isSearching) R.string.tasks_no_match_hint else R.string.tasks_empty_hint)
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

        if (isConnected) {
            updateSyncChip(
                getString(R.string.sync_synced),
                R.color.success,
                R.color.success_container
            )
        } else {
            updateSyncChip(
                getString(R.string.sync_status_offline),
                R.color.danger,
                R.color.muted_container
            )
        }
    }

    private fun navigateToAddTask() {
        findNavController().navigate(R.id.action_taskListFragment_to_addTaskFragment)
    }

    override fun onDestroyView() {
        super.onDestroyView()
        _binding = null
    }
}