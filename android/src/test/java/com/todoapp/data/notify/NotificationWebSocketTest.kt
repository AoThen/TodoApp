package com.todoapp.data.notify

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import org.json.JSONObject
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class NotificationWebSocketTest {

    private val context = ApplicationProvider.getApplicationContext<Context>()
    private val token = "test-access-token"

    private lateinit var externalScope: CoroutineScope

    @Before
    fun setUp() {
        externalScope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    }

    @After
    fun tearDown() {
        externalScope.cancel()
    }

    @Suppress("UNCHECKED_CAST")
    private fun handlersOf(ws: NotificationWebSocket): MutableMap<String, (JSONObject) -> Unit> {
        val field = NotificationWebSocket::class.java.getDeclaredField("messageHandlers")
        field.isAccessible = true
        return field.get(ws) as MutableMap<String, (JSONObject) -> Unit>
    }

    @Test
    fun `初始状态未连接`() {
        val ws = NotificationWebSocket(context, token, encryptionEnabled = false, externalScope = externalScope)
        assertFalse(ws.isConnected())
    }

    @Test
    fun `onMessage 注册指定类型的处理器`() {
        val ws = NotificationWebSocket(context, token, encryptionEnabled = false, externalScope = externalScope)
        var called = false
        ws.onMessage("task_created") { called = true }

        val handlers = handlersOf(ws)
        assertTrue(handlers.containsKey("task_created"))
        // 直接调用注册的处理器，验证注册的就是传入的 lambda
        handlers["task_created"]!!.invoke(JSONObject())
        assertTrue(called)
    }

    @Test
    fun `offMessage 注销处理器`() {
        val ws = NotificationWebSocket(context, token, encryptionEnabled = false, externalScope = externalScope)
        ws.onMessage("task_updated") { }
        ws.offMessage("task_updated")
        assertFalse(handlersOf(ws).containsKey("task_updated"))
    }

    @Test
    fun `offMessage 注销不存在的类型也安全`() {
        val ws = NotificationWebSocket(context, token, encryptionEnabled = false, externalScope = externalScope)
        ws.offMessage("not_registered")
        assertFalse(handlersOf(ws).containsKey("not_registered"))
    }

    @Test
    fun `同类型重复注册覆盖旧处理器`() {
        val ws = NotificationWebSocket(context, token, encryptionEnabled = false, externalScope = externalScope)
        var first = false
        var second = false
        ws.onMessage("sync_conflict") { first = true }
        ws.onMessage("sync_conflict") { second = true }

        val handlers = handlersOf(ws)
        assertEquals(1, handlers.size)
        handlers["sync_conflict"]!!.invoke(JSONObject())
        assertFalse(first)
        assertTrue(second)
    }

    @Test
    fun `未连接时 disconnect 安全`() {
        val ws = NotificationWebSocket(context, token, encryptionEnabled = false, externalScope = externalScope)
        ws.disconnect()
        assertFalse(ws.isConnected())
    }

    @Test
    fun `disconnect 后可重复调用`() {
        val ws = NotificationWebSocket(context, token, encryptionEnabled = false, externalScope = externalScope)
        ws.disconnect()
        ws.disconnect()
        assertFalse(ws.isConnected())
    }

    @Test
    fun `使用外部 scope 时 disconnect 不取消外部 scope`() {
        val ws = NotificationWebSocket(context, token, encryptionEnabled = true, externalScope = externalScope)
        ws.disconnect()
        assertTrue(externalScope.isActive)
    }
}
