package com.todoapp.data.backup

import com.google.gson.Gson
import com.google.gson.JsonSyntaxException
import com.todoapp.data.local.Task

/** 导出/导入的任务条目：只保留用户可读字段，不暴露本地自增 id、serverId 等实现细节 */
data class ExportedTask(
    val title: String? = null,
    val description: String? = null,
    val status: String? = null,
    val priority: String? = null,
    val dueAt: String? = null,
    val createdAt: String? = null,
    val updatedAt: String? = null,
    val completedAt: String? = null
)

/** 导入解析结果：valid 为可交给 createTask 的规范化数据，invalid 为被跳过的行数 */
data class ImportResult(
    val valid: List<ExportedTask>,
    val invalid: Int
)

/** 本地 Room 直读直写的任务导入导出：导出 JSON/CSV，导入 JSON 后走既有离线创建链路同步上云 */
class ImportExportManager {

    private val gson = Gson()

    fun toJson(tasks: List<Task>): String {
        val exported = tasks.map { t ->
            ExportedTask(
                title = t.title,
                description = t.description,
                status = t.status,
                priority = t.priority,
                dueAt = t.dueAt,
                createdAt = t.createdAt,
                updatedAt = t.updatedAt,
                completedAt = t.completedAt
            )
        }
        return gson.toJson(exported)
    }

    fun toCsv(tasks: List<Task>): String {
        val header = listOf(
            "title", "description", "status", "priority",
            "due_at", "created_at", "updated_at", "completed_at"
        )
        val sb = StringBuilder()
        // UTF-8 BOM：便于 Excel/WPS 直接识别编码
        sb.append('\uFEFF')
        sb.append(header.joinToString(","))
        sb.append('\n')
        for (t in tasks) {
            val row = listOf(
                t.title,
                t.description,
                t.status,
                t.priority,
                t.dueAt ?: "",
                t.createdAt,
                t.updatedAt,
                t.completedAt ?: ""
            )
            sb.append(row.joinToString(",") { escapeCsv(it) })
            sb.append('\n')
        }
        return sb.toString()
    }

    /** 解析导入的 JSON；整体不是数组时抛 [IllegalArgumentException]，由调用方统一提示 */
    fun parseImport(json: String): ImportResult {
        val exported: List<ExportedTask> = try {
            gson.fromJson(json, Array<ExportedTask>::class.java)?.toList() ?: emptyList()
        } catch (e: JsonSyntaxException) {
            throw IllegalArgumentException("invalid import payload", e)
        }
        val valid = mutableListOf<ExportedTask>()
        var invalid = 0
        for (item in exported) {
            if (item.title.isNullOrBlank()) {
                invalid++
                continue
            }
            valid += item.copy(
                title = item.title,
                status = normalizeStatus(item.status),
                priority = normalizePriority(item.priority)
            )
        }
        return ImportResult(valid, invalid)
    }

    private fun normalizeStatus(status: String?): String =
        if (status != null && status in ALLOWED_STATUS) status else "todo"

    private fun normalizePriority(priority: String?): String =
        if (priority != null && priority in ALLOWED_PRIORITY) priority else "medium"

    private fun escapeCsv(value: String): String =
        if (value.contains(',') || value.contains('"') || value.contains('\n') || value.contains('\r')) {
            "\"" + value.replace("\"", "\"\"") + "\""
        } else {
            value
        }

    companion object {
        private val ALLOWED_STATUS = setOf("todo", "in_progress", "done", "archived")
        private val ALLOWED_PRIORITY = setOf("low", "medium", "high")

        fun exportFileName(extension: String): String =
            "todoapp-tasks-" +
                java.time.LocalDate.now().format(java.time.format.DateTimeFormatter.BASIC_ISO_DATE) +
                "." + extension
    }
}
