package com.todoapp.data.notify

import android.content.Context
import com.todoapp.data.crypto.AesGcmManager
import com.todoapp.data.crypto.KeyStorage
import kotlinx.coroutines.*
import okhttp3.*
import okio.ByteString
import org.json.JSONObject
import java.util.concurrent.TimeUnit
import kotlin.math.pow

class NotificationWebSocket(
    private val context: Context,
    private val token: String,
    private val encryptionEnabled: Boolean = true,
    externalScope: CoroutineScope? = null
) {
    private val okHttpClient = OkHttpClient.Builder()
        .pingInterval(30, TimeUnit.SECONDS)
        .readTimeout(0, TimeUnit.MILLISECONDS)
        .build()

    private var webSocket: WebSocket? = null
    private var ready: Boolean = false
    private val messageHandlers = java.util.concurrent.ConcurrentHashMap<String, (JSONObject) -> Unit>()
    private var reconnectAttempts: Int = 0
    private val maxReconnectAttempts: Int = com.todoapp.config.SyncConfig.MAX_RECONNECT_ATTEMPTS
    private var reconnectJob: Job? = null
    // Store external scope reference to check if we should manage its lifecycle
    private val managedScope: Boolean = externalScope == null
    // Use external scope if provided, otherwise create a managed scope
    private val scope: CoroutineScope = externalScope ?: CoroutineScope(Dispatchers.IO + SupervisorJob())

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
        // 防止重叠建连泄漏：先静默取消旧连接（cancel 不会触发 onClosed/onFailure 重连回调）
        webSocket?.cancel()
        webSocket = null
        ready = false

        val serverAddress = getServerAddress()
        val encryptionParam = if (encryptionEnabled) "&encryption=true" else ""
        // 与服务端协议保持一致：https 用 wss，http 用 ws
        val wsProtocol = if (com.todoapp.data.remote.RetrofitClient.getBaseUrl().startsWith("https")) "wss://" else "ws://"
        val url = "$wsProtocol$serverAddress/ws?token=$token$encryptionParam"
        val request = Request.Builder()
            .url(url)
            .build()

        webSocket = okHttpClient.newWebSocket(request, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                android.util.Log.d(Config.TAG, "WebSocket connected to $serverAddress")
                reconnectAttempts = 0
                sendHandshake()
            }

            override fun onMessage(webSocket: WebSocket, text: String) {
                handleMessage(text, false)
            }

            override fun onMessage(webSocket: WebSocket, bytes: ByteString) {
                handleMessage(bytes.toByteArray(), true)
            }

            override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
                android.util.Log.d(Config.TAG, "WebSocket closing: $code - $reason")
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                android.util.Log.d(Config.TAG, "WebSocket closed: $code - $reason")
                ready = false
                handleReconnect()
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
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
                    val bytes = data as? ByteArray
                        ?: throw IllegalArgumentException("加密消息载荷类型异常")
                    // 解密接收到的数据
                    try {
                        val key = KeyStorage.getKey(context.applicationContext)
                        if (key != null) {
                            AesGcmManager.decryptBytes(bytes, key).toString(Charsets.UTF_8)
                        } else {
                            throw Exception("无法获取解密密钥")
                        }
                    } catch (e: Exception) {
                        android.util.Log.e(Config.TAG, "解密失败", e)
                        throw e
                    }
                } else {
                    data as? String
                        ?: throw IllegalArgumentException("明文消息载荷类型异常")
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
                if (encrypt && encryptionEnabled) {
                    val key = KeyStorage.getKey(context.applicationContext)
                    if (key != null) {
                        val encryptedBytes = AesGcmManager.encryptBytes(messageStr.toByteArray(Charsets.UTF_8), key)
                        webSocket?.send(okio.ByteString.of(*encryptedBytes))
                    } else {
                        android.util.Log.w(Config.TAG, "No encryption key available, sending unencrypted")
                        webSocket?.send(messageStr)
                    }
                } else {
                    webSocket?.send(messageStr)
                }
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
            // 指数退避（2s、4s、8s...），封顶 60s，避免频繁重试耗电
            val delay = minOf(
                (2.0.pow(reconnectAttempts) * 1000).toLong(),
                com.todoapp.config.SyncConfig.MAX_RECONNECT_DELAY_MS
            )

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
        // Only cancel the scope if it was created internally
        if (managedScope) {
            scope.cancel()
        }
        webSocket?.close(1000, "User disconnect")
        webSocket = null
        ready = false
        reconnectAttempts = 0
    }

    fun isConnected(): Boolean {
        return webSocket != null && ready
    }

    /**
     * 重连是否已耗尽放弃：处于该状态的实例不会再自行恢复，
     * 调用方（NotificationSocketController）应销毁并重建连接
     */
    fun isReconnectExhausted(): Boolean {
        return reconnectAttempts >= maxReconnectAttempts && !isConnected()
    }
}