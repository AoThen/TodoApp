package com.todoapp.data.sync

import com.google.gson.Gson
import com.google.gson.JsonObject

/**
 * 将本地 delta payload（Task 的 camelCase JSON 字符串）转换为服务端 /sync 接口期望的扁平字段结构。
 *
 * 服务端约定（见 handlers_sync.go）：
 * - insert: 读取 title/description/status/priority
 * - update/delete: 读取 id（服务端任务 ID）与 title/description/status/priority/due_at
 */
object SyncPayloadConverter {

    private val gson = Gson()

    fun payloadForServer(payloadJson: String): Map<String, Any> {
        return try {
            val obj: JsonObject = gson.fromJson(payloadJson, JsonObject::class.java)
                ?: return emptyMap()
            val result = mutableMapOf<String, Any>()

            // 服务端 update/delete 需要服务端任务 ID（本地自增 id 与服务端无关）
            if (obj.has("serverId") && !obj.get("serverId").isJsonNull) {
                result["id"] = obj.get("serverId").asLong
            }
            for (key in listOf("title", "description", "status", "priority")) {
                if (obj.has(key) && !obj.get(key).isJsonNull) {
                    result[key] = obj.get(key).asString
                }
            }
            // Task 实体字段为 camelCase，服务端读取 snake_case 的 due_at（空串表示清除）
            if (obj.has("dueAt") && !obj.get("dueAt").isJsonNull) {
                result["due_at"] = obj.get("dueAt").asString
            }
            result
        } catch (e: Exception) {
            emptyMap()
        }
    }
}
