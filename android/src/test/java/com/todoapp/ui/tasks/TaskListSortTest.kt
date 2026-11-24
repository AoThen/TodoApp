package com.todoapp.ui.tasks

import com.todoapp.data.local.Task
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/**
 * 手动排序纯函数测试：sortTasks 排序模式与 computeSortOrderForPosition 中点插入。
 * computeSortOrderForPosition 的入参约定为"已完成移动"的列表（与 ItemTouchHelper.onMove
 * 提交后的 currentList 一致），被拖任务位于 position 位置。
 */
@RunWith(RobolectricTestRunner::class)
class TaskListSortTest {

    private fun task(
        localId: String,
        sortOrder: Double = 0.0,
        createdAt: String = "2026-01-01T00:00:00Z",
        priority: String = "medium",
        updatedAt: String = "2026-01-01T00:00:00Z"
    ) = Task(
        localId = localId,
        userId = "u1",
        title = localId,
        sortOrder = sortOrder,
        createdAt = createdAt,
        updatedAt = updatedAt,
        lastModified = createdAt,
        priority = priority
    )

    @Test
    fun `manual 模式按 sortOrder 升序 同值按创建时间倒序`() {
        val a = task("a", sortOrder = 10.0, createdAt = "2026-01-03T00:00:00Z")
        val b = task("b", sortOrder = 5.0, createdAt = "2026-01-02T00:00:00Z")
        val c1 = task("c1", sortOrder = 0.0, createdAt = "2026-01-01T00:00:00Z")
        val c2 = task("c2", sortOrder = 0.0, createdAt = "2026-01-02T00:00:00Z")

        val sorted = TaskListViewModel.sortTasks(listOf(a, b, c1, c2), TaskListViewModel.SORT_MODE_MANUAL)

        assertEquals(listOf("c2", "c1", "b", "a"), sorted.map { it.localId })
    }

    @Test
    fun `默认模式按创建时间倒序`() {
        val old = task("old", createdAt = "2026-01-01T00:00:00Z")
        val new = task("new", createdAt = "2026-01-02T00:00:00Z")

        val sorted = TaskListViewModel.sortTasks(listOf(old, new), TaskListViewModel.SORT_MODE_CREATED)

        assertEquals(listOf("new", "old"), sorted.map { it.localId })
    }

    @Test
    fun `优先级模式 high 在前`() {
        val low = task("low", priority = "low")
        val high = task("high", priority = "high")
        val medium = task("medium", priority = "medium")

        val sorted = TaskListViewModel.sortTasks(listOf(low, high, medium), TaskListViewModel.SORT_MODE_PRIORITY)

        assertEquals(listOf("high", "medium", "low"), sorted.map { it.localId })
    }

    @Test
    fun `拖到中间取相邻中点`() {
        val a = task("a", sortOrder = 0.0)
        val b = task("b", sortOrder = 10.0)
        val c = task("c", sortOrder = 20.0)

        // [a, b, c] 把 c 拖到 a、b 之间 → 移动后 [a, c, b]，c 在位置 1 → 新值 5.0
        val result = TaskListViewModel.computeSortOrderForPosition(listOf(a, c, b), position = 1)

        assertEquals(5.0, result)
    }

    @Test
    fun `拖到顶部取首项减 1000`() {
        val a = task("a", sortOrder = 5.0)
        val b = task("b", sortOrder = 10.0)

        // [a, b] 把 b 拖到顶部 → [b, a]，b 在位置 0 → follow=a → 5-1000
        val result = TaskListViewModel.computeSortOrderForPosition(listOf(b, a), position = 0)

        assertEquals(-995.0, result)
    }

    @Test
    fun `拖到底部取末项加 1000`() {
        val a = task("a", sortOrder = 5.0)
        val b = task("b", sortOrder = 10.0)

        // [a, b] 把 a 拖到底部 → [b, a]，a 在位置 1 → prev=b → 10+1000
        val result = TaskListViewModel.computeSortOrderForPosition(listOf(b, a), position = 1)

        assertEquals(1010.0, result)
    }

    @Test
    fun `越界位置返回 null`() {
        val a = task("a")
        val b = task("b")

        assertNull(TaskListViewModel.computeSortOrderForPosition(listOf(a, b), 5))
        assertNull(TaskListViewModel.computeSortOrderForPosition(listOf(a, b), -1))
    }
}
