package com.todoapp.ui.tasks

import com.todoapp.data.local.Task
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * TaskListViewModel 搜索过滤的纯 JVM 单测（无需 Robolectric/Android 框架）。
 */
class TaskListViewModelFilterTest {

    private fun task(id: Long, title: String, description: String = "") = Task(
        id = id,
        localId = "local-$id",
        userId = "user-1",
        title = title,
        description = description,
        createdAt = "2026-11-22T00:00:00Z",
        updatedAt = "2026-11-22T00:00:00Z",
        lastModified = "2026-11-22T00:00:00Z"
    )

    private val tasks = listOf(
        task(1, "完成季度 OKR 评审材料", "整理 Q4 进度"),
        task(2, "修复 Delta 同步冲突检测", "client_version 比对"),
        task(3, "编写 WebSocket 心跳机制")
    )

    @Test
    fun `blank query returns all tasks`() {
        assertEquals(tasks, TaskListViewModel.filterTasks(tasks, ""))
        assertEquals(tasks, TaskListViewModel.filterTasks(tasks, "   "))
    }

    @Test
    fun `filter matches title ignoring case`() {
        val result = TaskListViewModel.filterTasks(tasks, "websocket")
        assertEquals(listOf(tasks[2]), result)
    }

    @Test
    fun `filter matches description`() {
        val result = TaskListViewModel.filterTasks(tasks, "client_version")
        assertEquals(listOf(tasks[1]), result)
    }

    @Test
    fun `filter returns empty when nothing matches`() {
        assertEquals(emptyList<Task>(), TaskListViewModel.filterTasks(tasks, "不存在的关键词"))
    }
}
