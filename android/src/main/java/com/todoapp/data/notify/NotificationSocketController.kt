package com.todoapp.data.notify

import android.content.Context
import com.todoapp.data.crypto.KeyStorage
import com.todoapp.data.remote.RetrofitClient
import com.todoapp.utils.JwtUtils
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import org.json.JSONObject

/**
 * 通知实时推送接线：
 * - 登录且已配对时建立与服务端 /ws 的长连接
 * - 接收 type=notification 推送，落库并通过系统通知渠道展示
 * - 生命周期由调用方控制：进前台/登录/配对成功后 start，退后台 stop
 */
object NotificationSocketController {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var socket: NotificationWebSocket? = null
    private var activeToken: String? = null

    @Synchronized
    fun start(context: Context) {
        val appContext = context.applicationContext
        val token = RetrofitClient.getAccessToken(appContext)
        // 未登录或未配对时无法建立加密通道
        if (token.isEmpty() || !KeyStorage.hasValidPairing(appContext)) {
            return
        }
        // 同一 token 已有连接（含退避重连中）时复用，避免重复建连
        if (socket != null && activeToken == token) {
            return
        }
        socket?.disconnect()

        val userId = JwtUtils.parseUserId(token).toIntOrNull() ?: return
        val manager = NotificationManager(appContext, userId, scope)
        val ws = NotificationWebSocket(appContext, token)
        ws.onMessage("notification") { json -> handleNotification(manager, json) }

        socket = ws
        activeToken = token
        ws.connect()
    }

    @Synchronized
    fun stop() {
        socket?.disconnect()
        socket = null
        activeToken = null
    }

    private fun handleNotification(manager: NotificationManager, json: JSONObject) {
        val data = json.optJSONObject("data") ?: return
        scope.launch {
            try {
                manager.createNotification(
                    type = data.optString("type", "system"),
                    title = data.optString("title", "通知"),
                    content = data.optString("content", ""),
                    priority = data.optString("priority", "normal")
                )
            } catch (e: Exception) {
                android.util.Log.e("NotificationSocket", "处理推送通知失败", e)
            }
        }
    }
}
