package com.todoapp.ui.tasks

import android.content.Context
import com.todoapp.data.local.Task
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
class AddTaskViewModelTest {

    private lateinit var viewModel: AddTaskViewModel
    private lateinit var taskRepository: TaskRepository
    private lateinit var context: Context

    private val task = mockk<Task>()

    @Before
    fun setup() {
        Dispatchers.setMain(UnconfinedTestDispatcher())
        taskRepository = mockk()
        context = mockk(relaxed = true) {
            every { getString(any<Int>()) } returns "校验提示"
        }
        viewModel = AddTaskViewModel(taskRepository)
    }

    @After
    fun tearDown() {
        Dispatchers.resetMain()
        unmockkAll()
    }

    @Test
    fun `初始状态为 Idle 且表单无效`() {
        assertTrue(viewModel.uiState.value is AddTaskState.Idle)
        assertFalse(viewModel.formState.value.isFormValid)
    }

    @Test
    fun `标题为空时校验失败`() {
        viewModel.validateTitle("   ", context)
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
    }

    @Test
    fun `标题描述都合法时表单有效`() {
        viewModel.validateTitle("合法标题", context)
        viewModel.validateDescription("合法描述", context)
        assertNull(viewModel.formState.value.titleError)
        assertNull(viewModel.formState.value.descriptionError)
        assertTrue(viewModel.formState.value.isFormValid)
    }

    @Test
    fun `标题错误时即使描述合法表单也无效`() {
        viewModel.validateDescription("合法描述", context)
        viewModel.validateTitle("", context)
        assertFalse(viewModel.formState.value.isFormValid)
    }

    @Test
    fun `validateForm 一次性校验所有字段`() {
        viewModel.validateForm(context)
        // 未填标题 → titleError 非空
        assertNotNull(viewModel.formState.value.titleError)
    }

    @Test
    fun `表单无效时 createTask 直接进入 Error 且不调用仓库`() {
        viewModel.createTask(context)
        assertTrue(viewModel.uiState.value is AddTaskState.Error)
        coVerify(exactly = 0) { taskRepository.createTask(any(), any(), any(), any(), any()) }
    }

    @Test
    fun `createTask 成功进入 Success 并重置表单`() {
        coEvery {
            taskRepository.createTask(any(), any(), any(), any(), any())
        } returns Result.Success(task)

        viewModel.validateTitle("买牛奶", context)
        viewModel.validateDescription("两盒", context)
        viewModel.createTask(context)

        val state = viewModel.uiState.value
        assertTrue(state is AddTaskState.Success)
        assertEquals(task, (state as AddTaskState.Success).task)
        assertFalse(viewModel.formState.value.isFormValid)
        assertEquals("", viewModel.getCurrentFormValues()["title"])
    }

    @Test
    fun `createTask 失败进入 Error`() {
        coEvery {
            taskRepository.createTask(any(), any(), any(), any(), any())
        } returns Result.Error(IllegalStateException("db down"))

        viewModel.validateTitle("买牛奶", context)
        viewModel.createTask(context)

        assertTrue(viewModel.uiState.value is AddTaskState.Error)
    }

    @Test
    fun `createTask 透传状态优先级与截止日期`() {
        coEvery {
            taskRepository.createTask(any(), any(), any(), any(), any())
        } returns Result.Success(task)

        viewModel.validateTitle("买牛奶", context)
        viewModel.validateDueDate("2026-12-01")
        viewModel.updateStatus("in_progress")
        viewModel.updatePriority("high")
        viewModel.createTask(context)

        val dueSlot = slot<String>()
        coVerify {
            taskRepository.createTask(
                title = "买牛奶",
                description = "",
                status = "in_progress",
                priority = "high",
                dueAt = capture(dueSlot)
            )
        }
        assertEquals("2026-12-01T00:00:00Z", dueSlot.captured)
    }

    @Test
    fun `createTask 空截止日期传 null`() {
        coEvery {
            taskRepository.createTask(any(), any(), any(), any(), any())
        } returns Result.Success(task)

        viewModel.validateTitle("买牛奶", context)
        viewModel.createTask(context)

        coVerify { taskRepository.createTask(any(), any(), any(), any(), dueAt = null) }
    }

    @Test
    fun `getCurrentFormValues 返回当前编辑值`() {
        viewModel.validateTitle("标题甲", context)
        viewModel.validateDescription("描述乙", context)
        viewModel.validateDueDate("2026-11-30")
        viewModel.updateStatus("done")
        viewModel.updatePriority("low")

        val values = viewModel.getCurrentFormValues()
        assertEquals("标题甲", values["title"])
        assertEquals("描述乙", values["description"])
        assertEquals("2026-11-30", values["due_date"])
        assertEquals("done", values["status"])
        assertEquals("low", values["priority"])
    }

    @Test
    fun `resetForm 清空所有字段与错误`() {
        viewModel.validateTitle("", context)
        viewModel.resetForm()

        val form = viewModel.formState.value
        assertNull(form.titleError)
        assertNull(form.descriptionError)
        assertFalse(form.isFormValid)
        assertEquals("", viewModel.getCurrentFormValues()["title"])
        assertEquals("todo", viewModel.getCurrentFormValues()["status"])
        assertEquals("medium", viewModel.getCurrentFormValues()["priority"])
    }
}
