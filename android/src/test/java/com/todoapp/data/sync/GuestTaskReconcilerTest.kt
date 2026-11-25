package com.todoapp.data.sync

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import com.todoapp.config.AppConfig
import com.todoapp.data.local.AppDatabase
import com.todoapp.data.local.DeltaChange
import com.todoapp.data.local.Task
import com.todoapp.data.remote.ApiService
import com.todoapp.data.remote.TaskResponse
import com.todoapp.data.remote.TasksPageData
import com.todoapp.data.remote.TasksPageResponse
import com.todoapp.data.remote.TasksPagination
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.mockk
import io.mockk.unmockkAll
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import retrofit2.Response

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class GuestTaskReconcilerTest {

    private lateinit var db: AppDatabase
    private lateinit var apiService: ApiService
    private lateinit var reconciler: GuestTaskReconciler
    private val context: Context = ApplicationProvider.getApplicationContext()

    @Before
    fun setUp() {
        db = AppDatabase.getInMemoryInstance(context)
        apiService = mockk()
        reconciler = GuestTaskReconciler(apiService, db)
    }

    @After
    fun tearDown() {
        db.close()
        unmockkAll()
    }

    private suspend fun insertGuestTask(
        localId: String,
        title: String,
        priority: String = "medium",
        dueAt: String? = null
    ): Task {
        val task = Task(
            localId = localId,
            userId = AppConfig.GUEST_USER_ID,
            title = title,
            priority = priority,
            dueAt = dueAt,
            createdAt = "2026-01-01T00:00:00Z",
            updatedAt = "2026-01-01T00:00:00Z",
            lastModified = "2026-01-01T00:00:00Z"
        )
        db.taskDao().insertTask(task)
        db.deltaQueueDao().insertDelta(
            DeltaChange(
                localId = localId,
                op = "insert",
                payload = "{}",
                clientVersion = 1,
                timestamp = "2026-01-01T00:00:00Z"
            )
        )
        return task
    }

    private fun serverTask(
        id: Long,
        title: String,
        priority: String? = "medium",
        dueAt: String? = null
    ) = TaskResponse(
        id = id,
        title = title,
        description = "",
        status = "todo",
        priority = priority,
        dueAt = dueAt,
        createdAt = "2026-01-01T00:00:00Z",
        updatedAt = "2026-01-02T00:00:00Z",
        completedAt = null
    )

    private fun pageResponse(
        tasks: List<TaskResponse>,
        page: Int = 1,
        pages: Int = 1
    ) = Response.success(
        TasksPageResponse(
            success = true,
            data = TasksPageData(
                tasks = tasks,
                pagination = TasksPagination(page = page, pageSize = 100, total = tasks.size, pages = pages)
            )
        )
    )

    @Test
    fun `无待上传新增时不请求服务端`() = runBlocking {
        val merged = reconciler.reconcile()

        assertEquals(0, merged)
        coVerify(exactly = 0) { apiService.getTasksPage(any(), any()) }
    }

    @Test
    fun `内容一致的服务端任务被认领且撤销上传`() = runBlocking {
        insertGuestTask("local-1", "买牛奶")
        coEvery { apiService.getTasksPage(1, 100) } returns pageResponse(listOf(serverTask(11, "买牛奶")))

        val merged = reconciler.reconcile()

        assertEquals(1, merged)
        val claimed = db.taskDao().getTaskByLocalId("local-1")
        assertNotNull(claimed)
        assertEquals(11L, claimed!!.serverId)
        // insert delta 已撤销，不会重复上传
        assertEquals(0, db.deltaQueueDao().getAllDeltas().size)
    }

    @Test
    fun `内容不一致不认领且保留待上传`() = runBlocking {
        insertGuestTask("local-1", "买牛奶", priority = "high")
        coEvery { apiService.getTasksPage(1, 100) } returns
            pageResponse(listOf(serverTask(11, "买牛奶", priority = "low")))

        val merged = reconciler.reconcile()

        assertEquals(0, merged)
        assertNull(db.taskDao().getTaskByLocalId("local-1")!!.serverId)
        assertEquals(1, db.deltaQueueDao().getAllDeltas().size)
    }

    @Test
    fun `同一条服务端任务只被认领一次`() = runBlocking {
        insertGuestTask("local-1", "买牛奶")
        insertGuestTask("local-2", "买牛奶")
        coEvery { apiService.getTasksPage(1, 100) } returns pageResponse(listOf(serverTask(11, "买牛奶")))

        val merged = reconciler.reconcile()

        assertEquals(1, merged)
        assertEquals(11L, db.taskDao().getTaskByLocalId("local-1")!!.serverId)
        assertNull(db.taskDao().getTaskByLocalId("local-2")!!.serverId)
        // 未被认领的任务保留 insert delta，稍后正常上传
        assertEquals(1, db.deltaQueueDao().getAllDeltas().size)
    }

    @Test
    fun `分页拉取服务端任务后对账`() = runBlocking {
        insertGuestTask("local-1", "买牛奶")
        coEvery { apiService.getTasksPage(1, 100) } returns
            pageResponse(listOf(serverTask(11, "买牛奶")), page = 1, pages = 2)
        coEvery { apiService.getTasksPage(2, 100) } returns
            pageResponse(emptyList(), page = 2, pages = 2)

        val merged = reconciler.reconcile()

        assertEquals(1, merged)
        coVerify { apiService.getTasksPage(2, 100) }
    }

    @Test
    fun `服务端请求失败时回退为原样上传`() = runBlocking {
        insertGuestTask("local-1", "买牛奶")
        coEvery { apiService.getTasksPage(1, 100) } throws java.net.ConnectException("network down")

        val merged = reconciler.reconcile()

        assertEquals(0, merged)
        assertNull(db.taskDao().getTaskByLocalId("local-1")!!.serverId)
        assertEquals(1, db.deltaQueueDao().getAllDeltas().size)
    }
}
