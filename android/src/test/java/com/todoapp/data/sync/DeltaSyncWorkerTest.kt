package com.todoapp.data.sync

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import androidx.work.ListenableWorker
import androidx.work.testing.TestListenableWorkerBuilder
import com.todoapp.data.local.AppDatabase
import com.todoapp.data.local.ConflictDao
import com.todoapp.data.local.DeltaChange
import com.todoapp.data.local.DeltaQueueDao
import com.todoapp.data.local.SyncMetaDao
import com.todoapp.data.local.Task
import com.todoapp.data.local.TaskDao
import com.todoapp.data.remote.ApiService
import com.todoapp.data.remote.ClientChange
import com.todoapp.data.remote.Conflict
import com.todoapp.data.remote.RetrofitClient
import com.todoapp.data.remote.ServerChange
import com.todoapp.data.remote.SyncResponse
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.every
import io.mockk.mockk
import io.mockk.mockkObject
import io.mockk.slot
import io.mockk.unmockkAll
import kotlinx.coroutines.runBlocking
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import retrofit2.Response

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class DeltaSyncWorkerTest {

    private lateinit var mockDb: AppDatabase
    private lateinit var mockTaskDao: TaskDao
    private lateinit var mockDeltaDao: DeltaQueueDao
    private lateinit var mockSyncMetaDao: SyncMetaDao
    private lateinit var mockConflictDao: ConflictDao
    private lateinit var mockApi: ApiService
    private val context: Context = ApplicationProvider.getApplicationContext()

    private fun jwt(payloadJson: String): String {
        // 真实 JWT payload 是 Base64URL 编码（无 padding），与 JwtUtils 的解码方式保持一致
        val payload = java.util.Base64.getUrlEncoder().encodeToString(payloadJson.toByteArray())
        return "header.$payload.signature"
    }

    private fun validJwt(userId: String = "42"): String = jwt(
        """{"user_id":"$userId","token_type":"access","exp":${System.currentTimeMillis() / 1000 + 3600}}"""
    )

    private val pendingDelta = DeltaChange(
        id = 11,
        localId = "local-1",
        op = "insert",
        payload = """{"localId":"local-1","title":"本地新建","status":"todo","priority":"high"}""",
        clientVersion = 1,
        timestamp = "2026-11-20T00:00:00Z"
    )

    @Before
    fun setUp() {
        mockkObject(AppDatabase.Companion)
        mockkObject(RetrofitClient)

        mockTaskDao = mockk(relaxed = true)
        mockDeltaDao = mockk(relaxed = true)
        mockSyncMetaDao = mockk(relaxed = true)
        mockConflictDao = mockk(relaxed = true)
        mockDb = mockk(relaxed = true)
        every { mockDb.taskDao() } returns mockTaskDao
        every { mockDb.deltaQueueDao() } returns mockDeltaDao
        every { mockDb.syncMetaDao() } returns mockSyncMetaDao
        every { mockDb.conflictDao() } returns mockConflictDao
        every { AppDatabase.getInstance(any()) } returns mockDb

        mockApi = mockk()
        every { RetrofitClient.getApiService(any()) } returns mockApi
        every { RetrofitClient.getAccessToken(any()) } returns validJwt()

        coEvery { mockDeltaDao.getAllDeltas() } returns emptyList()
        // relaxed mock 对可空返回值默认返回非 null mock，会误导幽灵合并分支；默认无幽灵任务
        coEvery { mockTaskDao.getUnsyncedTaskByTitle(any()) } returns null
    }

    @After
    fun tearDown() {
        unmockkAll()
    }

    private fun runWorker(): ListenableWorker.Result {
        val worker = TestListenableWorkerBuilder<DeltaSyncWorker>(context).build()
        // CoroutineWorker 的 suspend doWork() 直接返回 Result，无需 ListenableFuture.get()
        return runBlocking { worker.doWork() }
    }

    private fun stubSync(response: Response<SyncResponse>) {
        coEvery { mockApi.sync(any()) } returns response
    }

    private fun syncResponse(
        serverChanges: List<ServerChange> = emptyList(),
        clientChanges: List<ClientChange> = emptyList(),
        conflicts: List<Conflict> = emptyList(),
        lastSyncAt: String = "2026-11-20T01:00:00Z"
    ) = SyncResponse(
        serverChanges = serverChanges,
        clientChanges = clientChanges,
        lastSyncAt = lastSyncAt,
        conflicts = conflicts
    )

    @Test
    fun expiredToken_failsWithoutCallingApi() {
        every { RetrofitClient.getAccessToken(any()) } returns jwt(
            """{"user_id":"42","token_type":"access","exp":${System.currentTimeMillis() / 1000 - 100}}"""
        )

        assertEquals(ListenableWorker.Result.failure(), runWorker())
        coVerify(exactly = 0) { mockApi.sync(any()) }
    }

    @Test
    fun malformedToken_fails() {
        every { RetrofitClient.getAccessToken(any()) } returns "not-a-jwt"

        assertEquals(ListenableWorker.Result.failure(), runWorker())
        coVerify(exactly = 0) { mockApi.sync(any()) }
    }

    @Test
    fun nonAccessToken_fails() {
        every { RetrofitClient.getAccessToken(any()) } returns jwt(
            """{"user_id":"42","token_type":"refresh","exp":${System.currentTimeMillis() / 1000 + 3600}}"""
        )

        assertEquals(ListenableWorker.Result.failure(), runWorker())
        coVerify(exactly = 0) { mockApi.sync(any()) }
    }

    @Test
    fun noPendingDeltas_succeedsWithoutApiCall() {
        assertEquals(ListenableWorker.Result.success(), runWorker())
        coVerify(exactly = 0) { mockApi.sync(any()) }
    }

    @Test
    fun apiFailure_retries() {
        coEvery { mockDeltaDao.getAllDeltas() } returns listOf(pendingDelta)
        stubSync(
            Response.error(
                500,
                """{"error":"服务器内部错误"}""".toResponseBody("application/json".toMediaType())
            )
        )

        assertEquals(ListenableWorker.Result.retry(), runWorker())
    }

    @Test
    fun success_backfillsServerIdAndClearsConfirmedDeltas() {
        coEvery { mockDeltaDao.getAllDeltas() } returns listOf(pendingDelta)
        val localTask = Task(
            id = 5,
            localId = "local-1",
            userId = "42",
            title = "本地新建",
            createdAt = "t",
            updatedAt = "t",
            lastModified = "t"
        )
        coEvery { mockTaskDao.getTaskByLocalId("local-1") } returns localTask
        stubSync(
            Response.success(
                syncResponse(clientChanges = listOf(ClientChange(localId = "local-1", serverId = 100, op = "insert")))
            )
        )

        assertEquals(ListenableWorker.Result.success(), runWorker())

        val updated = slot<Task>()
        coVerify(exactly = 1) { mockTaskDao.updateTask(capture(updated)) }
        assertEquals(100L, updated.captured.serverId)
        coVerify(exactly = 1) { mockDeltaDao.deleteDelta(11L) }
        coVerify(exactly = 1) { mockApi.sync(any()) }
    }

    @Test
    fun success_appliesServerChangesWithUpsert() {
        coEvery { mockDeltaDao.getAllDeltas() } returns listOf(pendingDelta)
        val existing = Task(
            id = 5,
            localId = "local-2",
            serverId = 200,
            userId = "42",
            serverVersion = 3,
            title = "旧标题",
            description = "原描述",
            status = "todo",
            priority = "low",
            createdAt = "t0",
            updatedAt = "t0",
            lastModified = "t0"
        )
        coEvery { mockTaskDao.getTaskByServerId(200L) } returns existing
        coEvery { mockTaskDao.getTaskByServerId(300L) } returns null
        coEvery { mockTaskDao.getTaskByServerId(400L) } returns null
        stubSync(
            Response.success(
                syncResponse(
                    serverChanges = listOf(
                        ServerChange(
                            id = 200,
                            serverVersion = 4,
                            title = "服务端新标题",
                            description = null,
                            status = "in_progress",
                            priority = null,
                            dueAt = "2026-12-01",
                            updatedAt = "2026-11-20T02:00:00Z",
                            isDeleted = false
                        ),
                        ServerChange(
                            id = 300,
                            serverVersion = 1,
                            title = "其他设备新建",
                            updatedAt = "2026-11-20T02:00:00Z",
                            isDeleted = false
                        ),
                        ServerChange(
                            id = 400,
                            serverVersion = 2,
                            title = "已删除任务",
                            updatedAt = "2026-11-20T02:00:00Z",
                            isDeleted = true
                        )
                    )
                )
            )
        )

        assertEquals(ListenableWorker.Result.success(), runWorker())

        // 已存在：服务端非空字段覆盖，空字段保留本地值
        val merged = slot<Task>()
        coVerify(exactly = 1) { mockTaskDao.updateTask(capture(merged)) }
        assertEquals(200L, merged.captured.serverId)
        assertEquals(4, merged.captured.serverVersion)
        assertEquals("服务端新标题", merged.captured.title)
        assertEquals("原描述", merged.captured.description)
        assertEquals("in_progress", merged.captured.status)
        assertEquals("low", merged.captured.priority)
        assertEquals("2026-12-01", merged.captured.dueAt)
        assertEquals(false, merged.captured.isDeleted)

        // 本地不存在且未删除：作为其他设备的新建任务插入
        val inserted = slot<Task>()
        coVerify(exactly = 1) { mockTaskDao.insertTask(capture(inserted)) }
        assertEquals(300L, inserted.captured.serverId)
        assertEquals("其他设备新建", inserted.captured.title)
        assertEquals("42", inserted.captured.userId)

        // 本地不存在的删除变更：不产生任何写入
        coVerify(exactly = 0) { mockTaskDao.softDeleteTask(any(), any()) }
    }

    @Test
    fun success_mergesLocalGhostTaskForServerCreatedTask() {
        coEvery { mockDeltaDao.getAllDeltas() } returns listOf(pendingDelta)
        val ghost = Task(
            id = 9,
            localId = "local-ghost",
            userId = "42",
            title = "其他设备新建",
            createdAt = "t0",
            updatedAt = "t0",
            lastModified = "t0"
        )
        coEvery { mockTaskDao.getTaskByServerId(300L) } returns null
        coEvery { mockTaskDao.getUnsyncedTaskByTitle("其他设备新建") } returns ghost
        stubSync(
            Response.success(
                syncResponse(
                    serverChanges = listOf(
                        ServerChange(
                            id = 300,
                            serverVersion = 1,
                            title = "其他设备新建",
                            status = "todo",
                            updatedAt = "2026-11-20T02:00:00Z",
                            isDeleted = false
                        )
                    )
                )
            )
        )

        assertEquals(ListenableWorker.Result.success(), runWorker())

        // 同名本地未同步任务被合并：采用服务端身份，而不是重复插入
        val merged = slot<Task>()
        coVerify(exactly = 1) { mockTaskDao.updateTask(capture(merged)) }
        assertEquals("local-ghost", merged.captured.localId)
        assertEquals(300L, merged.captured.serverId)
        assertEquals(1, merged.captured.serverVersion)
        coVerify(exactly = 0) { mockTaskDao.insertTask(any()) }
    }

    @Test
    fun success_persistsConflictsAndSyncMeta() {
        coEvery { mockDeltaDao.getAllDeltas() } returns listOf(pendingDelta)
        stubSync(
            Response.success(
                syncResponse(
                    conflicts = listOf(
                        Conflict(localId = "local-1", serverId = 100, reason = "version_conflict", options = listOf("keep_local", "keep_server"))
                    )
                )
            )
        )

        assertEquals(ListenableWorker.Result.success(), runWorker())

        val conflict = slot<com.todoapp.data.local.Conflict>()
        coVerify(exactly = 1) { mockConflictDao.insertConflict(capture(conflict)) }
        assertEquals("local-1", conflict.captured.localId)
        assertEquals(100L, conflict.captured.serverId)
        assertEquals("version_conflict", conflict.captured.reason)
        assertEquals("keep_local,keep_server", conflict.captured.options)

        val meta = slot<com.todoapp.data.local.SyncMeta>()
        coVerify(exactly = 1) { mockSyncMetaDao.insertSyncMeta(capture(meta)) }
        assertEquals("42", meta.captured.userId)
        assertEquals("2026-11-20T01:00:00Z", meta.captured.lastSyncAt)
    }

    @Test
    fun syncRequestPayload_isFlatServerFormat() {
        coEvery { mockDeltaDao.getAllDeltas() } returns listOf(pendingDelta)
        val requestSlot = slot<com.todoapp.data.remote.SyncRequest>()
        stubSync(Response.success(syncResponse()))
        every { RetrofitClient.getAccessToken(any()) } returns validJwt()

        runWorker()

        coVerify { mockApi.sync(capture(requestSlot)) }
        val change = requestSlot.captured.changes.single()
        assertEquals("local-1", change.localId)
        assertEquals("insert", change.op)
        assertEquals(1, change.clientVersion)
        // insert 变更不应携带服务端 id（服务端 insert 只读 title/description/status/priority）
        org.junit.Assert.assertFalse(change.payload.containsKey("id"))
        assertEquals("本地新建", change.payload["title"])
        assertEquals("high", change.payload["priority"])
        assertEquals("todo", change.payload["status"])
    }
}
