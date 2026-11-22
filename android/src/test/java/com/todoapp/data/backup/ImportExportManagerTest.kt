package com.todoapp.data.backup

import com.todoapp.data.local.Task
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class ImportExportManagerTest {

    private val manager = ImportExportManager()

    private fun task(
        title: String = "任务",
        description: String = "",
        status: String = "todo",
        priority: String = "medium",
        dueAt: String? = null
    ) = Task(
        localId = "local-$title",
        userId = "42",
        title = title,
        description = description,
        status = status,
        priority = priority,
        dueAt = dueAt,
        createdAt = "2026-01-01T00:00:00Z",
        updatedAt = "2026-01-02T00:00:00Z",
        lastModified = "2026-01-02T00:00:00Z"
    )

    @Test
    fun json_roundTrip() {
        val tasks = listOf(
            task(title = "写报告", description = "周报", status = "in_progress", priority = "high", dueAt = "2026-12-01"),
            task(title = "带引号\"与逗号,的任务")
        )

        val parsed = manager.parseImport(manager.toJson(tasks))

        assertEquals(0, parsed.invalid)
        assertEquals(2, parsed.valid.size)
        assertEquals("写报告", parsed.valid[0].title)
        assertEquals("周报", parsed.valid[0].description)
        assertEquals("in_progress", parsed.valid[0].status)
        assertEquals("high", parsed.valid[0].priority)
        assertEquals("2026-12-01", parsed.valid[0].dueAt)
        assertEquals("带引号\"与逗号,的任务", parsed.valid[1].title)
    }

    @Test
    fun parseImport_skipsBlankTitleAndCoercesEnums() {
        val json = """
            [
              {"title":"  ","status":"done"},
              {"description":"缺少 title 的行"},
              {"title":"正常任务","status":"奇怪状态","priority":"超高"},
              {"title":"无状态任务"}
            ]
        """.trimIndent()

        val parsed = manager.parseImport(json)

        // 空白标题与缺失 title 的行均被跳过，不中断整份导入
        assertEquals(2, parsed.invalid)
        assertEquals(2, parsed.valid.size)
        assertEquals("todo", parsed.valid[0].status)
        assertEquals("medium", parsed.valid[0].priority)
        assertEquals("todo", parsed.valid[1].status)
    }

    @Test
    fun parseImport_rejectsNonArrayPayload() {
        assertThrows(IllegalArgumentException::class.java) {
            manager.parseImport("""{"title":"不是数组"}""")
        }
    }

    @Test
    fun parseImport_emptyArrayYieldsNoTasks() {
        val parsed = manager.parseImport("[]")
        assertEquals(0, parsed.valid.size)
        assertEquals(0, parsed.invalid)
    }

    @Test
    fun csv_escapesSpecialCharactersAndAddsBom() {
        val csv = manager.toCsv(listOf(task(title = "带,逗号", description = "含\"引号\"与\n换行")))

        assertTrue(csv.startsWith("\uFEFF"))
        val body = csv.removePrefix("\uFEFF")
        assertEquals(
            "title,description,status,priority,due_at,created_at,updated_at,completed_at",
            body.lines().first()
        )
        // 含逗号/引号/换行的字段被引号包裹，内部引号翻倍转义（转义字段可跨行，需对整段断言）
        assertTrue(body.contains("\"带,逗号\""))
        assertTrue(body.contains("\"含\"\"引号\"\"与\n换行\""))
    }

    @Test
    fun exportFileName_usesDateAndExtension() {
        val name = ImportExportManager.exportFileName("json")
        assertTrue(name.startsWith("todoapp-tasks-"))
        assertTrue(name.endsWith(".json"))
    }
}
