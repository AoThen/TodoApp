package com.todoapp.config

import android.content.Context
import com.todoapp.BuildConfig

object AppConfig {
    private const val PREFS_NAME = "app_config"
    private const val KEY_BASE_URL = "base_url"
    private const val KEY_IS_FIRST_LAUNCH = "is_first_launch"

    const val DEFAULT_BASE_URL = "http://10.0.2.2:8080/api/v1/"
    const val WEB_SOCKET_PATH = "/ws"

    private var baseUrl: String = DEFAULT_BASE_URL

    fun initialize(context: Context) {
        val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        baseUrl = prefs.getString(KEY_BASE_URL, DEFAULT_BASE_URL) ?: DEFAULT_BASE_URL
    }

    fun getBaseUrl(): String = baseUrl

    fun setBaseUrl(context: Context, url: String) {
        baseUrl = if (url.endsWith("/")) {
            if (!url.endsWith("/api/v1/")) {
                url + "api/v1/"
            } else {
                url
            }
        } else {
            url + "/api/v1/"
        }

        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            .edit()
            .putString(KEY_BASE_URL, baseUrl)
            .apply()
    }

    fun getWebSocketUrl(token: String, encryptionEnabled: Boolean = true): String {
        val serverAddress = getServerAddress()
        val encryptionParam = if (encryptionEnabled) "&encryption=true" else ""
        return "ws://$serverAddress$WEB_SOCKET_PATH?token=$token$encryptionParam"
    }

    private fun getServerAddress(): String {
        return try {
            val url = java.net.URL(baseUrl)
            val host = url.host
            val port = if (url.port > 0) url.port else if (url.protocol == "https") 443 else 80
            "$host:$port"
        } catch (e: Exception) {
            "10.0.2.2:8080"
        }
    }

    fun isFirstLaunch(context: Context): Boolean {
        return context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            .getBoolean(KEY_IS_FIRST_LAUNCH, true)
    }

    fun setFirstLaunchComplete(context: Context) {
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            .edit()
            .putBoolean(KEY_IS_FIRST_LAUNCH, false)
            .apply()
    }
}

object SyncConfig {
    const val SYNC_INTERVAL_MINUTES = 15L
    const val MAX_RETRY_ATTEMPTS = 3
    const val CONNECTION_TIMEOUT_SECONDS = 30L
    const val READ_TIMEOUT_SECONDS = 30L
    const val WRITE_TIMEOUT_SECONDS = 30L

    const val PING_INTERVAL_SECONDS = 30
    const val MAX_RECONNECT_ATTEMPTS = 5

    const val CONNECTION_CHECK_INTERVAL_SECONDS = 30
    const val CONNECTION_CHECK_TIMEOUT_MS = 5000
}

object EncryptionConfig {
    const val KEY_SIZE = 256
    const val GCM_IV_LENGTH = 12
    const val GCM_TAG_LENGTH = 128
    const val ALGORITHM = "AES/GCM/NoPadding"
}

object TaskConfig {
    const val MAX_TITLE_LENGTH = 200
    const val MAX_DESCRIPTION_LENGTH = 2000
    const val DEFAULT_STATUS = "todo"
    const val DEFAULT_PRIORITY = "medium"

    val PRIORITIES = listOf("low", "medium", "high")
    val STATUSES = listOf("todo", "in_progress", "completed", "archived")
}

object NotificationConfig {
    const val PRIORITY_URGENT = "urgent"
    const val PRIORITY_HIGH = "high"
    const val PRIORITY_NORMAL = "normal"
    const val PRIORITY_LOW = "low"

    val PRIORITIES = listOf(PRIORITY_URGENT, PRIORITY_HIGH, PRIORITY_NORMAL, PRIORITY_LOW)
}
