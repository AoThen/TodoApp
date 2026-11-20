package com.todoapp.data.sync

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

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
    fun `非法 JSON 返回空 map 不抛异常`() {
        assertTrue(SyncPayloadConverter.payloadForServer("not-json").isEmpty())
        assertTrue(SyncPayloadConverter.payloadForServer("").isEmpty())
    }
}
