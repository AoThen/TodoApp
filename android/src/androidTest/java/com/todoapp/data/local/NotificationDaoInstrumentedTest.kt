package com.todoapp.data.local

import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import kotlinx.coroutines.test.runTest
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class NotificationDaoInstrumentedTest {

    private lateinit var db: AppDatabase
    private lateinit var dao: NotificationDao

    private fun notification(id: Int, isRead: Boolean = false, type: String = "task_assigned") =
        Notification(
            userId = 1,
            type = type,
            title = "通知$id",
            content = "内容$id",
            priority = "normal",
            isRead = isRead,
            createdAt = "2026-01-01T00:00:0${id % 10}Z"
        )

    @Before
    fun setup() {
        db = AppDatabase.getInMemoryInstance(ApplicationProvider.getApplicationContext())
        dao = db.notificationDao()
    }

    @After
    fun tearDown() {
        db.close()
    }

    @Test
    fun insertAndPaginatedQuery() = runTest {
        dao.insertNotifications((1..5).map { notification(it) })

        val page1 = dao.getNotificationsPaginated(1, 2, 0)
        val page2 = dao.getNotificationsPaginated(1, 2, 2)
        assertEquals(2, page1.size)
        assertEquals(2, page2.size)
        assertTrue(page1.none { it.id == page2.first().id })
    }

    @Test
    fun unreadCount只统计未读() = runTest {
        dao.insertNotifications(listOf(notification(1), notification(2), notification(3, isRead = true)))
        assertEquals(2, dao.getUnreadCount(1))
    }

    @Test
    fun markAsRead更新已读状态与时间() = runTest {
        val id = dao.insertNotification(notification(1))
        dao.markAsRead(id, "2026-01-02T08:00:00Z")
        assertEquals(0, dao.getUnreadCount(1))
        assertEquals("2026-01-02T08:00:00Z", dao.getNotificationById(id)!!.readAt)
    }

    @Test
    fun markAllAsRead返回影响行数() = runTest {
        dao.insertNotifications(listOf(notification(1), notification(2)))
        val updated = dao.markAllAsRead(1, "2026-01-02T08:00:00Z")
        assertEquals(2, updated)
        assertEquals(0, dao.getUnreadCount(1))
    }

    @Test
    fun unreadCountByType按类型过滤() = runTest {
        dao.insertNotifications(
            listOf(notification(1, type = "task_assigned"), notification(2, type = "sync"))
        )
        assertEquals(1, dao.getUnreadCountByType(1, listOf("task_assigned")))
        assertEquals(2, dao.getUnreadCountByType(1, null))
    }

    @Test
    fun deleteNotification与clearAll() = runTest {
        val id1 = dao.insertNotification(notification(1))
        dao.insertNotification(notification(2))
        dao.deleteNotification(id1)
        assertNull(dao.getNotificationById(id1))
        assertEquals(1, dao.clearAllNotifications(1))
        assertEquals(0, dao.getUnreadCount(1))
    }

    @Test
    fun otherUser数据不受影响() = runTest {
        val n = notification(1).copy(userId = 2)
        dao.insertNotification(n)
        assertEquals(0, dao.getUnreadCount(1))
        assertEquals(1, dao.getUnreadCount(2))
    }
}
