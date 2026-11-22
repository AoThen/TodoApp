package com.todoapp.data.remote

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKeys
import okhttp3.Cookie
import okhttp3.CookieJar
import okhttp3.HttpUrl

/**
 * 持久化 CookieJar：服务端通过 HttpOnly Cookie 下发 refresh token，
 * 后续 /auth/refresh 请求必须携带该 Cookie 才能刷新 access token。
 *
 * Cookie 值属于长期凭据，落盘前使用 EncryptedSharedPreferences 加密；
 * 历史明文 cookie 文件直接废弃（不迁移，相关用户重新登录即可）。
 */
class PersistentCookieJar(context: Context) : CookieJar {

    private val appContext: Context = context.applicationContext
    private val prefs: SharedPreferences = createEncryptedPrefs(appContext)
    private val cache = LinkedHashMap<String, Cookie>()

    init {
        runCatching { appContext.deleteSharedPreferences(LEGACY_COOKIE_PREFS) }
        prefs.getStringSet(KEY_COOKIES, emptySet())?.forEach { entry ->
            val separatorIndex = entry.indexOf('|')
            if (separatorIndex > 0) {
                val host = entry.substring(0, separatorIndex)
                val header = entry.substring(separatorIndex + 1)
                if (host.isNotEmpty()) {
                    runCatching {
                        Cookie.parse(
                            HttpUrl.Builder().scheme("https").host(host).build(),
                            header
                        )
                    }.getOrNull()?.let { cache[it.name] = it }
                }
            }
        }
    }

    private fun createEncryptedPrefs(appContext: Context): SharedPreferences {
        return try {
            val masterKeyAlias = MasterKeys.getOrCreate(MasterKeys.AES256_GCM_SPEC)
            EncryptedSharedPreferences.create(
                COOKIE_PREFS,
                masterKeyAlias,
                appContext,
                EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
                EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
            )
        } catch (e: Exception) {
            android.util.Log.e("PersistentCookieJar", "加密存储不可用，降级为普通存储", e)
            appContext.getSharedPreferences(COOKIE_PREFS, Context.MODE_PRIVATE)
        }
    }

    override fun saveFromResponse(url: HttpUrl, cookies: List<Cookie>) {
        synchronized(cache) {
            var changed = false
            cookies.forEach { cookie ->
                if (cache[cookie.name] != cookie) {
                    cache[cookie.name] = cookie
                    changed = true
                }
            }
            if (changed) persist()
        }
    }

    override fun loadForRequest(url: HttpUrl): List<Cookie> {
        synchronized(cache) {
            val now = System.currentTimeMillis()
            val expired = cache.values.filter { it.expiresAt <= now }
            if (expired.isNotEmpty()) {
                expired.forEach { cache.remove(it.name) }
                persist()
            }
            return cache.values.filter { it.matches(url) }
        }
    }

    fun clear() {
        synchronized(cache) {
            cache.clear()
            prefs.edit().remove(KEY_COOKIES).apply()
        }
    }

    private fun persist() {
        // 会话 cookie（非 persistent）不落盘，仅内存持有
        val entries = cache.values.filter { it.persistent }.map { cookie ->
            "${cookie.domain}|${cookie.toString()}"
        }.toSet()
        prefs.edit().putStringSet(KEY_COOKIES, entries).apply()
    }

    companion object {
        private const val COOKIE_PREFS = "todoapp_cookies_encrypted"
        private const val LEGACY_COOKIE_PREFS = "todoapp_cookies"
        private const val KEY_COOKIES = "cookies"
    }
}
