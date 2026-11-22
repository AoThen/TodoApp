package com.todoapp.data.remote

import android.content.Context
import android.content.SharedPreferences
import okhttp3.Cookie
import okhttp3.CookieJar
import okhttp3.HttpUrl

/**
 * 持久化 CookieJar：服务端通过 HttpOnly Cookie 下发 refresh token，
 * 后续 /auth/refresh 请求必须携带该 Cookie 才能刷新 access token。
 */
class PersistentCookieJar(context: Context) : CookieJar {

    private val prefs: SharedPreferences =
        context.getSharedPreferences(COOKIE_PREFS, Context.MODE_PRIVATE)
    private val cache = LinkedHashMap<String, Cookie>()

    init {
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
        val entries = cache.values.map { cookie ->
            "${cookie.domain}|${cookie.toString()}"
        }.toSet()
        prefs.edit().putStringSet(KEY_COOKIES, entries).apply()
    }

    companion object {
        private const val COOKIE_PREFS = "todoapp_cookies"
        private const val KEY_COOKIES = "cookies"
    }
}
