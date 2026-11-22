package com.todoapp.data.repository

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import com.google.gson.Gson
import com.todoapp.data.local.AppDatabase
import com.todoapp.data.local.Task
import com.todoapp.data.remote.RetrofitClient
import com.todoapp.utils.Result
import io.mockk.every
import io.mockk.mockkObject
import io.mockk.unmockkAll
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class TaskRepositoryTest {

    private lateinit var db: AppDatabase
    private lateinit var repository: TaskRepository
    private val context: Context = ApplicationProvider.getApplicationContext()

    private fun fakeJwt(userId: String = "42"): String {
        // 真实 JWT payload 是 Base64URL 编码（无 padding），与 JwtUtils 的解码方式保持一致
        val payload = java.util.Base64.getUrlEncoder().encodeToString(
            """{"user_id":"$userId","token_type":"access","exp":${System.currentTimeMillis() / 1000 + 3600}}""".toByteArray()
        )
        return "header.$payload.signature"
    }

    @Before
    fun setUp() {
        mockkObject(RetrofitClient)
        every { RetrofitClient.getAccessToken(any()) } returns fakeJwt()
        db = AppDatabase.getInMemoryInstance(context)
        repository = TaskRepository(db.taskDao(), db.deltaQueueDao(), db.syncMetaDao(), context)
    }

    @After
    fun tearDown() {
        db.close()
        unmockkAll()
    }

    private suspend fun insertLocalTask(title: String = "本地任务", status: String = "todo"): Task {
        val task = Task(
            localId = "local-$title",
            userId = "42",
            title = title,
            status = status,
            createdAt = "2026-01-01T00:00:00Z",
            updatedAt = "2026-01-01T00:00:00Z",
            lastModified = "2026-01-01T00:00:00Z"
        )
        val id = db.taskDao().insertTask(task)
        return task.copy(id = id)
    }

    @Test
    fun createTask_insertsTaskAndEnqueuesInsertDelta() = runBlocking {
        val result = repository.createTask("写报告", description = "周报", priority = "high")

        val task = (result as Result.Success).data
        assertEquals("42", task.userId)
        assertEquals("写报告", task.title)
        assertEquals("high", task.priority)
        assertEquals("todo", task.status)

        val stored = db.taskDao().getTaskByLocalId(task.localId)
        assertNotNull(stored)
        assertEquals(task.title, stored!!.title)

        val deltas = db.deltaQueueDao().getAllDeltas()
        assertEquals(1, deltas.size)
        assertEquals("insert", deltas[0].op)
        assertEquals(task.localId, deltas[0].localId)
        assertEquals(1, deltas[0].clientVersion)
        val payload = Gson().fromJson(deltas[0].payload, Map::class.java)
        assertEquals("写报告", payload["title"])
        assertEquals(task.localId, payload["localId"])
    }

    @Test
    fun createTask_usesUserIdFromAccessToken() = runBlocking {
        every { RetrofitClient.getAccessToken(any()) } returns fakeJwt("77")
        val task = (repository.createTask("任务B") as Result.Success).data
        assertEquals("77", task.userId)
    }

    @Test
    fun deleteTask_marksDeletedAndEnqueuesDeleteDelta() = runBlocking {
        val task = insertLocalTask()
        val before = db.deltaQueueDao().getAllDeltas().size

        val result = repository.deleteTask(task)

        assertTrue(result is Result.Success)
        val stored = db.taskDao().getTaskByLocalId(task.localId)
        assertNotNull(stored)
        assertEquals(true, stored!!.isDeleted)
        val deltas = db.deltaQueueDao().getAllDeltas()
        assertEquals(before + 1, deltas.size)
        assertEquals("delete", deltas.last().op)
        assertEquals(task.localId, deltas.last().localId)
    }

    @Test
    fun toggleTaskCompletion_togglesStatusAndTracksCompletedAt() = runBlocking {
        val task = insertLocalTask(status = "todo")

        repository.toggleTaskCompletion(task)
        val done = db.taskDao().getTaskByLocalId(task.localId)!!
        assertEquals("done", done.status)
        assertNotNull(done.completedAt)
        assertEquals("update", db.deltaQueueDao().getAllDeltas().last().op)

        repository.toggleTaskCompletion(done)
        val reopened = db.taskDao().getTaskByLocalId(task.localId)!!
        assertEquals("todo", reopened.status)
        assertNull(reopened.completedAt)
    }

    @Test
    fun updateTask_refreshesTimestamp() = runBlocking {
        val task = insertLocalTask()
        val updated = task.copy(title = "改后的标题")

        repository.updateTask(updated)

        val stored = db.taskDao().getTaskByLocalId(task.localId)!!
        assertEquals("改后的标题", stored.title)
        assertTrue(stored.updatedAt >= task.updatedAt)
    }

    @Test
    fun getTaskById_returnsErrorWhenMissing() = runBlocking {
        val result = repository.getTaskById(99999)
        assertTrue(result is Result.Error)
    }

    @Test
    fun deltaQueueCrud_roundTrips() = runBlocking {
        insertLocalTask()
        val pending = (repository.getPendingDeltas() as Result.Success).data
        assertTrue(pending.isEmpty())

        repository.deleteTask(insertLocalTask("带队列"))
        val after = (repository.getPendingDeltas() as Result.Success).data
        assertEquals(1, after.size)

        repository.clearDelta(after[0].id)
        assertTrue((repository.getPendingDeltas() as Result.Success).data.isEmpty())
    }

    @Test
    fun syncMeta_upsertsPerUser() = runBlocking {
        assertNull((repository.getSyncMeta("42") as Result.Success).data)

        repository.updateSyncMeta("42", "2026-11-20T00:00:00Z")
        val meta = (repository.getSyncMeta("42") as Result.Success).data
        assertNotNull(meta)
        assertEquals("2026-11-20T00:00:00Z", meta!!.lastSyncAt)

        repository.updateSyncMeta("42", "2026-11-20T12:00:00Z")
        val updated = (repository.getSyncMeta("42") as Result.Success).data
        assertEquals("2026-11-20T12:00:00Z", updated!!.lastSyncAt)
    }
}
