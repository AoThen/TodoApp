package com.todoapp.data.sync

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * delta payload → 服务端字段映射测试。
 * 服务端 /sync 按下划线键读取（due_at/sort_order/recurrence_type 等），
 * 本地 Task 实体序列化为 camelCase，转换器必须完成键名翻译且不丢新字段。
 */
class SyncPayloadConverterTest {

    @Test
    fun `insert payload 转换为服务端扁平结构`() {
        val json = """
            {"id":0,"localId":"abc","serverId":null,"userId":"1","serverVersion":0,
             "title":"买牛奶","description":"两盒","status":"todo","priority":"high",
             "dueAt":"2026-12-01","createdAt":"2026-11-20T00:00:00Z","updatedAt":"2026-11-20T00:00:00Z",
             "completedAt":null,"isDeleted":false,"isArchived":false,"archivedAt":null,
             "lastModified":"2026-11-20T00:00:00Z"}
        """.trimIndent()

        val payload = SyncPayloadConverter.payloadForServer(json)

        assertEquals("买牛奶", payload["title"])
        assertEquals("两盒", payload["description"])
        assertEquals("todo", payload["status"])
        assertEquals("high", payload["priority"])
        assertEquals("2026-12-01", payload["due_at"])
        // serverId 为 null 时不携带 id，服务端按 local_id 走插入分支
        assertFalse(payload.containsKey("id"))
    }

    @Test
    fun `update 与 delete payload 携带服务端 id`() {
        val json = """
            {"id":7,"localId":"abc","serverId":42,"userId":"1","serverVersion":3,
             "title":"改标题","description":"","status":"done","priority":"low",
             "dueAt":null,"createdAt":"2026-11-20T00:00:00Z","updatedAt":"2026-11-20T01:00:00Z",
             "completedAt":null,"isDeleted":false,"isArchived":false,"archivedAt":null,
             "lastModified":"2026-11-20T01:00:00Z"}
        """.trimIndent()

        val payload = SyncPayloadConverter.payloadForServer(json)

        assertEquals(42L, payload["id"])
        assertEquals("改标题", payload["title"])
        assertEquals("done", payload["status"])
        // dueAt 为 null 时不下发 due_at（保持原值）
        assertFalse(payload.containsKey("due_at"))
    }

    @Test
    fun `dueAt 为空串时保留空串语义用于清除截止日期`() {
        val json = """{"serverId":9,"title":"t","description":"","status":"todo","priority":"medium","dueAt":""}"""
        val payload = SyncPayloadConverter.payloadForServer(json)
        assertEquals("", payload["due_at"])
    }

    @Test
    fun `手动排序与循环字段翻译为服务端 snake_case 键`() {
        val json = """
            {"serverId":42,"title":"任务","status":"todo","priority":"high",
             "dueAt":"2026-01-01T00:00:00Z","completedAt":"",
             "sortOrder":3.5,"recurrenceType":"weekly","recurrenceSeriesId":"series-1"}
        """.trimIndent()

        val payload = SyncPayloadConverter.payloadForServer(json)

        assertEquals(42L, payload["id"])
        assertEquals("2026-01-01T00:00:00Z", payload["due_at"])
        assertEquals("", payload["completed_at"])
        assertEquals(3.5, payload["sort_order"])
        assertEquals("weekly", payload["recurrence_type"])
        assertEquals("series-1", payload["recurrence_series_id"])
        // 不应把本地 camelCase 键原样透传
        assertFalse(payload.containsKey("dueAt"))
        assertFalse(payload.containsKey("sortOrder"))
        assertFalse(payload.containsKey("recurrenceType"))
    }

    @Test
    fun `snake_case 滞留 payload 同样兼容（升级过渡期）`() {
        val json = """
            {"server_id":7,"title":"任务","due_at":"2026-02-01T00:00:00Z",
             "sort_order":1.5,"recurrence_type":"daily"}
        """.trimIndent()

        val payload = SyncPayloadConverter.payloadForServer(json)

        assertEquals(7L, payload["id"])
        assertEquals("2026-02-01T00:00:00Z", payload["due_at"])
        assertEquals(1.5, payload["sort_order"])
        assertEquals("daily", payload["recurrence_type"])
    }

    @Test
    fun `缺省新字段不出现在结果中（服务端保持原值）`() {
        val json = """{"serverId":1,"title":"仅标题"}"""

        val payload = SyncPayloadConverter.payloadForServer(json)

        assertEquals(1L, payload["id"])
        assertEquals("仅标题", payload["title"])
        assertFalse(payload.containsKey("sort_order"))
        assertFalse(payload.containsKey("recurrence_type"))
        assertFalse(payload.containsKey("recurrence_series_id"))
    }

    @Test
    fun `非法 JSON 返回空 map 不抛异常`() {
        assertTrue(SyncPayloadConverter.payloadForServer("not-json").isEmpty())
        assertTrue(SyncPayloadConverter.payloadForServer("").isEmpty())
    }
}
