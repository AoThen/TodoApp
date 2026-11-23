package com.todoapp.ui.tasks

import android.content.Context
import com.todoapp.R
import com.todoapp.data.local.Task
import com.todoapp.data.local.TaskDao
import com.todoapp.data.repository.TaskRepository
import com.todoapp.utils.Result
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.every
import io.mockk.mockk
import io.mockk.slot
import io.mockk.unmockkAll
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@OptIn(ExperimentalCoroutinesApi::class)
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class TaskDetailViewModelTest {

    private lateinit var viewModel: TaskDetailViewModel
    private lateinit var taskDao: TaskDao
    private lateinit var taskRepository: TaskRepository
    private lateinit var context: Context

    private val task = Task(
        id = 1,
        localId = "local-1",
        serverId = null,
        userId = "user-1",
        serverVersion = 0,
        title = "买牛奶",
        description = "两盒",
        status = "todo",
        priority = "medium",
        dueAt = "2026-11-25T00:00:00Z",
        createdAt = "2026-11-20T00:00:00Z",
        updatedAt = "2026-11-20T00:00:00Z",
        lastModified = "2026-11-20T00:00:00Z"
    )

    @Before
    fun setup() {
        Dispatchers.setMain(UnconfinedTestDispatcher())
        taskDao = mockk()
        taskRepository = mockk()
        context = mockk(relaxed = true) {
            every { getString(R.string.task_not_found) } returns "任务不存在"
            every { getString(R.string.task_load_failed, any()) } returns "加载失败"
            every { getString(R.string.task_title_empty) } returns "标题不能为空"
            every { getString(R.string.task_title_too_long) } returns "标题过长"
            every { getString(R.string.task_description_too_long) } returns "描述过长"
            every { getString(R.string.task_fix_form_errors) } returns "请修复表单错误"
            every { getString(R.string.task_update_failed, any()) } returns "更新失败"
            every { getString(R.string.task_delete_failed, any()) } returns "删除失败"
        }
        viewModel = TaskDetailViewModel(taskDao, taskRepository)
    }

    @After
    fun tearDown() {
        Dispatchers.resetMain()
        unmockkAll()
    }

    @Test
    fun `初始状态为 Idle 且无未保存变化`() {
        assertTrue(viewModel.uiState.value is TaskDetailState.Idle)
        assertFalse(viewModel.hasUnsavedChanges)
        assertFalse(viewModel.formState.value.isFormValid)
    }

    @Test
    fun `loadTask 成功进入 Success 并回填表单`() {
        coEvery { taskRepository.getTaskByLocalId("local-1") } returns Result.Success(task)

        viewModel.loadTask("local-1", context)

        val state = viewModel.uiState.value
        assertTrue(state is TaskDetailState.Success)
        assertEquals(task, (state as TaskDetailState.Success).task)
        assertEquals("买牛奶", viewModel.getCurrentFormValues()["title"])
        assertEquals("两盒", viewModel.getCurrentFormValues()["description"])
        assertEquals("2026-11-25", viewModel.getCurrentFormValues()["due_date"])
        assertFalse(viewModel.hasUnsavedChanges)
    }

    @Test
    fun `loadTask 任务不存在时进入 Error 提示任务不存在`() {
        coEvery { taskRepository.getTaskByLocalId("missing") } returns Result.Error(NoSuchElementException())

        viewModel.loadTask("missing", context)

        val state = viewModel.uiState.value
        assertTrue(state is TaskDetailState.Error)
        assertEquals("任务不存在", (state as TaskDetailState.Error).message)
    }

    @Test
    fun `loadTask 其他异常时进入 Error 带异常消息`() {
        coEvery { taskRepository.getTaskByLocalId("local-1") } returns Result.Error(IllegalStateException("db down"))

        viewModel.loadTask("local-1", context)

        val state = viewModel.uiState.value
        assertTrue(state is TaskDetailState.Error)
        assertEquals("加载失败", (state as TaskDetailState.Error).message)
    }

    @Test
    fun `标题为空时校验失败`() {
        viewModel.validateTitle("", context)

        assertNotNull(viewModel.formState.value.titleError)
        assertFalse(viewModel.formState.value.isFormValid)
    }

    @Test
    fun `标题超过200字时校验失败`() {
        viewModel.validateTitle("a".repeat(201), context)

        assertNotNull(viewModel.formState.value.titleError)
    }

    @Test
    fun `标题恰好200字时校验通过`() {
        viewModel.validateTitle("a".repeat(200), context)

        assertNull(viewModel.formState.value.titleError)
    }

    @Test
    fun `描述超过2000字时校验失败`() {
        viewModel.validateDescription("d".repeat(2001), context)

        assertNotNull(viewModel.formState.value.descriptionError)
        assertFalse(viewModel.formState.value.isFormValid)
    }

    @Test
    fun `validateForm 一次性校验标题与描述`() {
        viewModel.validateForm("合法标题", "d".repeat(2001), "2026-11-25", context)

        val form = viewModel.formState.value
        assertNull(form.titleError)
        assertNotNull(form.descriptionError)
        assertFalse(form.isFormValid)
    }

    @Test
    fun `表单无效时 updateTask 进入 Error 且不调用仓库`() {
        coEvery { taskRepository.getTaskByLocalId("local-1") } returns Result.Success(task)
        viewModel.loadTask("local-1", context)
        viewModel.validateTitle("", context)

        viewModel.updateTask(context)

        assertTrue(viewModel.uiState.value is TaskDetailState.Error)
        coVerify(exactly = 0) { taskRepository.updateTask(any()) }
    }

    @Test
    fun `updateTask 成功进入 Success 并清除未保存标记`() {
        coEvery { taskRepository.getTaskByLocalId("local-1") } returns Result.Success(task)
        coEvery { taskRepository.updateTask(any()) } returns Result.Success(Unit)
        viewModel.loadTask("local-1", context)
        viewModel.validateTitle("新标题", context)
        assertTrue(viewModel.hasUnsavedChanges)

        viewModel.updateTask(context)

        val state = viewModel.uiState.value
        assertTrue(state is TaskDetailState.Success)
        assertEquals("新标题", (state as TaskDetailState.Success).task.title)
        assertFalse(viewModel.hasUnsavedChanges)
    }

    @Test
    fun `清空截止日期时 updateTask 传 dueAt 为 null`() {
        coEvery { taskRepository.getTaskByLocalId("local-1") } returns Result.Success(task)
        coEvery { taskRepository.updateTask(any()) } returns Result.Success(Unit)
        viewModel.loadTask("local-1", context)
        viewModel.validateDueDate("")

        viewModel.updateTask(context)

        val taskSlot = slot<Task>()
        coVerify { taskRepository.updateTask(capture(taskSlot)) }
        assertNull(taskSlot.captured.dueAt)
    }

    @Test
    fun `updateTask 失败进入 Error`() {
        coEvery { taskRepository.getTaskByLocalId("local-1") } returns Result.Success(task)
        coEvery { taskRepository.updateTask(any()) } returns Result.Error(IllegalStateException("db down"))
        viewModel.loadTask("local-1", context)

        viewModel.updateTask(context)

        val state = viewModel.uiState.value
        assertTrue(state is TaskDetailState.Error)
        assertEquals("更新失败", (state as TaskDetailState.Error).message)
    }

    @Test
    fun `deleteTask 成功进入 Success`() {
        coEvery { taskRepository.getTaskByLocalId("local-1") } returns Result.Success(task)
        coEvery { taskRepository.deleteTask(any()) } returns Result.Success(Unit)
        viewModel.loadTask("local-1", context)

        viewModel.deleteTask(context)

        assertTrue(viewModel.uiState.value is TaskDetailState.Success)
    }

    @Test
    fun `deleteTask 失败进入 Error`() {
        coEvery { taskRepository.getTaskByLocalId("local-1") } returns Result.Success(task)
        coEvery { taskRepository.deleteTask(any()) } returns Result.Error(IllegalStateException("db down"))
        viewModel.loadTask("local-1", context)

        viewModel.deleteTask(context)

        val state = viewModel.uiState.value
        assertTrue(state is TaskDetailState.Error)
        assertEquals("删除失败", (state as TaskDetailState.Error).message)
    }

    @Test
    fun `revertChanges 恢复原任务值并清除未保存标记`() {
        coEvery { taskRepository.getTaskByLocalId("local-1") } returns Result.Success(task)
        viewModel.loadTask("local-1", context)
        viewModel.validateTitle("改了标题", context)
        viewModel.updateStatus("done")
        assertTrue(viewModel.hasUnsavedChanges)

        viewModel.revertChanges()

        assertEquals("买牛奶", viewModel.getCurrentFormValues()["title"])
        assertEquals("两盒", viewModel.getCurrentFormValues()["description"])
        assertEquals("2026-11-25", viewModel.getCurrentFormValues()["due_date"])
        assertEquals("todo", viewModel.getCurrentFormValues()["status"])
        assertFalse(viewModel.hasUnsavedChanges)
        assertTrue(viewModel.formState.value.isFormValid)
    }

    @Test
    fun `清空截止日期后 revertChanges 恢复原日期`() {
        coEvery { taskRepository.getTaskByLocalId("local-1") } returns Result.Success(task)
        viewModel.loadTask("local-1", context)
        viewModel.validateDueDate("")
        assertTrue(viewModel.hasUnsavedChanges)

        viewModel.revertChanges()

        assertEquals("2026-11-25", viewModel.getCurrentFormValues()["due_date"])
        assertFalse(viewModel.hasUnsavedChanges)
    }

    @Test
    fun `修改字段后标记未保存变化`() {
        coEvery { taskRepository.getTaskByLocalId("local-1") } returns Result.Success(task)
        viewModel.loadTask("local-1", context)
        assertFalse(viewModel.hasUnsavedChanges)

        viewModel.updatePriority("high")

        assertTrue(viewModel.hasUnsavedChanges)
    }

    @Test
    fun `getCurrentFormValues 返回当前编辑值`() {
        coEvery { taskRepository.getTaskByLocalId("local-1") } returns Result.Success(task)
        viewModel.loadTask("local-1", context)

        val values = viewModel.getCurrentFormValues()

        assertEquals("买牛奶", values["title"])
        assertEquals("两盒", values["description"])
        assertEquals("2026-11-25", values["due_date"])
        assertEquals("todo", values["status"])
        assertEquals("medium", values["priority"])
    }
}
