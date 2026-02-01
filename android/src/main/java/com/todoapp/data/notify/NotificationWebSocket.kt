package com.todoapp.data.notify

import android.content.Context
import kotlinx.coroutines.*
import okhttp3.*
import okio.ByteString
import org.json.JSONObject
import java.util.*
import java.util.concurrent.TimeUnit

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
    private var aesGcmManager: AesGcmManager? = null
    private val messageHandlers = mutableMapOf<String, (JSONObject) -> Unit>()
    private var reconnectAttempts: Int = 0
    private val maxReconnectAttempts: Int = 5
    private var reconnectJob: Job? = null
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())

    private val companion = object {
        const val TAG = "NotificationWebSocket"
        const val SERVER_URL = "10.0.2.2:8080" // Android 模拟器访问宿主机
    }

    fun connect() {
        if (encryptionEnabled) {
            aesGcmManager = AesGcmManager(context)
        }

        val encryptionParam = if (encryptionEnabled) "&encryption=true" else ""
        val url = "ws://${companion.SERVER_URL}/ws?token=$token$encryptionParam"
        val request = Request.Builder()
            .url(url)
            .build()

        webSocket = okHttpClient.newWebSocket(request, object : WebSocketListener() {
            override fun onOpen(ws: WebSocket, response: Response) {
                android.util.Log.d(companion.TAG, "WebSocket connected")
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
                android.util.Log.d(companion.TAG, "WebSocket closing: $code - $reason")
            }

            override fun onClosed(ws: WebSocket, code: Int, reason: String) {
                android.util.Log.d(companion.TAG, "WebSocket closed: $code - $reason")
                ready = false
                handleReconnect()
            }

            override fun onFailure(ws: WebSocket, t: Throwable, response: Response?) {
                android.util.Log.e(companion.TAG, "WebSocket error", t)
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
        send(handshake, false) // 握手不加密
    }

    private fun handleMessage(data: Any, isEncrypted: Boolean) {
        scope.launch {
            try {
                val jsonString = if (isEncrypted) {
                    val bytes = data as ByteArray
                    decryptMessage(bytes)
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
                        android.util.Log.e(companion.TAG, "Server error: $error")
                    }
                    else -> {
                        val handler = messageHandlers[type]
                        handler?.invoke(json)
                    }
                }
            } catch (e: Exception) {
                android.util.Log.e(companion.TAG, "Failed to handle message", e)
            }
        }
    }

    private fun handleHandshake(json: JSONObject) {
        val encryptionEnabled = json.optBoolean("encryption_enabled")
        if (encryptionEnabled != this.encryptionEnabled) {
            android.util.Log.e(companion.TAG, "Encryption negotiation failed")
            disconnect()
            return
        }
        ready = true
        android.util.Log.d(companion.TAG, "WebSocket handshake completed, encryption: $encryptionEnabled")
    }

    private fun encryptMessage(plaintext: String): ByteArray {
        if (!encryptionEnabled) {
            return plaintext.toByteArray()
        }

        return try {
            aesGcmManager?.encrypt(plaintext.toByteArray()) ?: plaintext.toByteArray()
        } catch (e: Exception) {
            android.util.Log.e(companion.TAG, "Failed to encrypt message", e)
            plaintext.toByteArray()
        }
    }

    private fun decryptMessage(ciphertext: ByteArray): String {
        if (!encryptionEnabled) {
            return String(ciphertext)
        }

        return try {
            val decrypted = aesGcmManager?.decrypt(ciphertext)
            String(decrypted ?: ciphertext)
        } catch (e: Exception) {
            android.util.Log.e(companion.TAG, "Failed to decrypt message", e)
            String(ciphertext)
        }
    }

    fun send(message: JSONObject, encrypt: Boolean = encryptionEnabled) {
        scope.launch {
            try {
                val messageStr = message.toString()

                if (encrypt && encryptionEnabled && ready) {
                    val encrypted = encryptMessage(messageStr)
                    webSocket?.send(ByteString.of(*encrypted))
                } else {
                    webSocket?.send(messageStr)
                }
            } catch (e: Exception) {
                android.util.Log.e(companion.TAG, "Failed to send message", e)
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

            android.util.Log.d(companion.TAG, "Reconnecting in ${delay}ms... (attempt $reconnectAttempts/$maxReconnectAttempts)")

            reconnectJob?.cancel()
            reconnectJob = scope.launch {
                delay(delay)
                connect()
            }
        } else {
            android.util.Log.e(companion.TAG, "Max reconnect attempts reached")
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
