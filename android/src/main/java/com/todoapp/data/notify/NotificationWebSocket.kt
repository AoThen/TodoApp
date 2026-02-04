package com.todoapp.data.notify

import android.content.Context
import kotlinx.coroutines.*
import okhttp3.*
import okio.ByteString
import org.json.JSONObject
import java.util.concurrent.TimeUnit
import kotlin.math.pow

class NotificationWebSocket(
    private val context: Context,
    private val token: String,
    private val encryptionEnabled: Boolean = true
) {
    private val okHttpClient = OkHttpClient.Builder()
        .pingInterval(30, TimeUnit.SECONDS)
        .readTimeout(0, TimeUnit.MILLISECONDS)
        .build()

    private var webSocket: WebSocket? = null
    private var ready: Boolean = false
    private val messageHandlers = mutableMapOf<String, (JSONObject) -> Unit>()
    private var reconnectAttempts: Int = 0
    private val maxReconnectAttempts: Int = 5
    private var reconnectJob: Job? = null
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())

    private object Config {
        val TAG = "NotificationWebSocket"
    }

    private fun getServerAddress(): String {
        val baseUrl = com.todoapp.data.remote.RetrofitClient.getBaseUrl()
        return try {
            val url = java.net.URL(baseUrl)
            val host = url.host
            val port = if (url.port > 0) url.port else if (url.protocol == "https") 443 else 80
            "$host:$port"
        } catch (e: Exception) {
            "10.0.2.2:8080"
        }
    }

    fun connect() {
        val serverAddress = getServerAddress()
        val encryptionParam = if (encryptionEnabled) "&encryption=true" else ""
        val url = "ws://$serverAddress/ws?token=$token$encryptionParam"
        val request = Request.Builder()
            .url(url)
            .build()

        webSocket = okHttpClient.newWebSocket(request, object : WebSocketListener() {
            override fun onOpen(ws: WebSocket, response: Response) {
                android.util.Log.d(Config.TAG, "WebSocket connected to $serverAddress")
                reconnectAttempts = 0
                sendHandshake()
            }

            override fun onMessage(ws: WebSocket, text: String) {
                handleMessage(text, false)
            }

            override fun onMessage(ws: WebSocket, bytes: ByteString) {
                handleMessage(bytes.toByteArray(), true)
            }

            override fun onClosing(ws: WebSocket, code: Int, reason: String) {
                android.util.Log.d(Config.TAG, "WebSocket closing: $code - $reason")
            }

            override fun onClosed(ws: WebSocket, code: Int, reason: String) {
                android.util.Log.d(Config.TAG, "WebSocket closed: $code - $reason")
                ready = false
                handleReconnect()
            }

            override fun onFailure(ws: WebSocket, t: Throwable, response: Response?) {
                android.util.Log.e(Config.TAG, "WebSocket error", t)
                ready = false
                handleReconnect()
            }
        })
    }

    private fun sendHandshake() {
        val handshake = JSONObject().apply {
            put("type", "handshake")
            put("encryption", encryptionEnabled)
            put("timestamp", System.currentTimeMillis())
        }
        send(handshake, false)
    }

    private fun handleMessage(data: Any, isEncrypted: Boolean) {
        scope.launch {
            try {
                val jsonString = if (isEncrypted) {
                    val bytes = data as ByteArray
                    String(bytes) // 简化：暂不解密，生产环境应实现解密
                } else {
                    data as String
                }

                val json = JSONObject(jsonString)
                val type = json.optString("type")

                when (type) {
                    "handshake" -> handleHandshake(json)
                    "pong" -> { }
                    "error" -> {
                        val error = json.optString("error")
                        android.util.Log.e(Config.TAG, "Server error: $error")
                    }
                    else -> {
                        val handler = messageHandlers[type]
                        handler?.invoke(json)
                    }
                }
            } catch (e: Exception) {
                android.util.Log.e(Config.TAG, "Failed to handle message", e)
            }
        }
    }

    private fun handleHandshake(json: JSONObject) {
        val encryptionEnabled = json.optBoolean("encryption_enabled")
        if (encryptionEnabled != this.encryptionEnabled) {
            android.util.Log.e(Config.TAG, "Encryption negotiation failed")
            disconnect()
            return
        }
        ready = true
        android.util.Log.d(Config.TAG, "WebSocket handshake completed, encryption: $encryptionEnabled")
    }

    fun send(message: JSONObject, encrypt: Boolean = encryptionEnabled) {
        scope.launch {
            try {
                val messageStr = message.toString()
                webSocket?.send(messageStr)
            } catch (e: Exception) {
                android.util.Log.e(Config.TAG, "Failed to send message", e)
            }
        }
    }

    fun onMessage(type: String, handler: (JSONObject) -> Unit) {
        messageHandlers[type] = handler
    }

    fun offMessage(type: String) {
        messageHandlers.remove(type)
    }

    private fun handleReconnect() {
        if (reconnectAttempts < maxReconnectAttempts) {
            reconnectAttempts++
            val delay = (2.0.pow(reconnectAttempts) * 1000).toLong()

            android.util.Log.d(Config.TAG, "Reconnecting in ${delay}ms... (attempt $reconnectAttempts/$maxReconnectAttempts)")

            reconnectJob?.cancel()
            reconnectJob = scope.launch {
                delay(delay)
                connect()
            }
        } else {
            android.util.Log.e(Config.TAG, "Max reconnect attempts reached")
        }
    }

    fun disconnect() {
        reconnectJob?.cancel()
        webSocket?.close(1000, "User disconnect")
        webSocket = null
        ready = false
        reconnectAttempts = 0
    }

    fun isConnected(): Boolean {
        return webSocket != null && ready
    }
}
