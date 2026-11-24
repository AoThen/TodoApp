package com.todoapp.data.sync

import com.google.gson.Gson
import com.google.gson.JsonObject

/**
 * 将本地 delta payload（Task 的 JSON 字符串）转换为服务端 /sync 接口期望的扁平字段结构。
 *
 * 服务端约定（见 handlers_sync.go）：
 * - insert: 读取 title/description/status/priority/due_at/sort_order/recurrence_*
 * - update/delete: 读取 id（服务端任务 ID）与 title/description/status/priority/due_at/... 等
 *
 * 键名兼容：历史队列中的 payload 为 camelCase（Task 实体原名），
 * 新字段一律以"两处取值"方式读取，避免升级后滞留 delta 丢字段。
 */
object SyncPayloadConverter {

    private val gson = Gson()

    fun payloadForServer(payloadJson: String): Map<String, Any> {
        return try {
            val obj: JsonObject = gson.fromJson(payloadJson, JsonObject::class.java)
                ?: return emptyMap()
            val result = mutableMapOf<String, Any>()

            // 服务端 update/delete 需要服务端任务 ID（本地自增 id 与服务端无关）
            firstOf(obj, "serverId", "server_id")?.let {
                result["id"] = it.asLong
            }
            for (key in listOf("title", "description", "status", "priority")) {
                firstOf(obj, key)?.let {
                    result[key] = it.asString
                }
            }
            // Task 实体字段为 camelCase，服务端读取 snake_case 的 due_at（空串表示清除）
            firstOf(obj, "dueAt", "due_at")?.let {
                result["due_at"] = it.asString
            }
            firstOf(obj, "completedAt", "completed_at")?.let {
                result["completed_at"] = it.asString
            }
            firstOf(obj, "sortOrder", "sort_order")?.let {
                result["sort_order"] = it.asDouble
            }
            firstOf(obj, "recurrenceType", "recurrence_type")?.let {
                result["recurrence_type"] = it.asString
            }
            firstOf(obj, "recurrenceSeriesId", "recurrence_series_id")?.let {
                result["recurrence_series_id"] = it.asString
            }
            result
        } catch (e: Exception) {
            emptyMap()
        }
    }

    /** 依次取第一个存在且非 null 的键 */
    private fun firstOf(obj: JsonObject, vararg keys: String) = keys.firstOrNull { key ->
        obj.has(key) && !obj.get(key).isJsonNull
    }?.let { obj.get(it) }
}
