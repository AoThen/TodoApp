package com.todoapp.data.local

import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runTest
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class TaskDaoInstrumentedTest {

    private lateinit var db: AppDatabase
    private lateinit var dao: TaskDao

    private fun task(title: String, localId: String = "local-$title", status: String = "todo") = Task(
        localId = localId,
        userId = "user-1",
        title = title,
        status = status,
        priority = "high",
        createdAt = "2026-01-0${title.length % 9 + 1}T00:00:00Z",
        updatedAt = "2026-01-01T00:00:00Z",
        lastModified = "2026-01-01T00:00:00Z"
    )

    @Before
    fun setup() {
        db = AppDatabase.getInMemoryInstance(ApplicationProvider.getApplicationContext())
        dao = db.taskDao()
    }

    @After
    fun tearDown() {
        db.close()
    }

    @Test
    fun insertAndGetByLocalId() = runTest {
        val id = dao.insertTask(task("写报告"))
        assertTrue(id > 0)
        val loaded = dao.getTaskByLocalId("local-写报告")
        assertNotNull(loaded)
        assertEquals("写报告", loaded!!.title)
        assertEquals("user-1", loaded.userId)
    }

    @Test
    fun getTaskByLocalId返回null当不存在() = runTest {
        assertNull(dao.getTaskByLocalId("nope"))
    }

    @Test
    fun updateTask修改标题与状态() = runTest {
        val id = dao.insertTask(task("原始标题"))
        val loaded = dao.getTaskById(id)!!
        dao.updateTask(loaded.copy(title = "更新后标题", status = "done"))
        val updated = dao.getTaskById(id)!!
        assertEquals("更新后标题", updated.title)
        assertEquals("done", updated.status)
    }

    @Test
    fun softDelete后列表不可见但单查仍返回() = runTest {
        val id = dao.insertTask(task("待删除"))
        dao.softDeleteTask(id, "2026-01-02T00:00:00Z")

        assertEquals(0, dao.getAllTasks().first().size)
        // 与后端软删语义一致：单查不过滤 isDeleted
        val single = dao.getTaskById(id)
        assertNotNull(single)
        assertEquals(true, single!!.isDeleted)
    }

    @Test
    fun getAllTasks按创建时间倒序且过滤已删除() = runTest {
        dao.insertTask(task("任务A"))
        dao.insertTask(task("任务B"))
        val deletedId = dao.insertTask(task("任务C"))
        dao.softDeleteTask(deletedId, "2026-01-02T00:00:00Z")

        val titles = dao.getAllTasks().first().map { it.title }
        assertEquals(listOf("任务B", "任务A"), titles)
    }

    @Test
    fun deleteAllTasks清空表() = runTest {
        dao.insertTask(task("任务A"))
        dao.insertTask(task("任务B"))
        dao.deleteAllTasks()
        assertEquals(0, dao.getAllTasks().first().size)
    }

    @Test
    fun insertOnConflictReplace覆盖同主键() = runTest {
        val original = task("替换前")
        val id = dao.insertTask(original)
        dao.insertTask(original.copy(id = id, title = "替换后"))
        assertEquals("替换后", dao.getTaskById(id)!!.title)
    }
}
