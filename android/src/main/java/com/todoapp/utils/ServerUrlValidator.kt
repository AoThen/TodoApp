package com.todoapp.utils

import android.net.Uri

/**
 * 服务器地址统一校验：配对二维码/手动输入与设置页修改共用同一套规则。
 * release 构建仅允许 https；debug 额外放行 http 以便本地/模拟器调试。
 */
object ServerUrlValidator {

    fun isValidServerUrl(url: String, allowHttp: Boolean): Boolean {
        val trimmed = url.trim()
        if (trimmed.isEmpty()) return false

        val uri = Uri.parse(trimmed)
        val scheme = uri.scheme?.lowercase()
        if (scheme != "https" && !(allowHttp && scheme == "http")) return false

        return !uri.host.isNullOrBlank()
    }

    /** 规范化为以 /api/v1/ 结尾的基址，与 AppConfig/RetrofitClient 的持久化规则一致，用于切换前后比较 */
    fun normalizeBaseUrl(url: String): String {
        val trimmed = url.trim()
        val withSlash = if (trimmed.endsWith("/")) trimmed else "$trimmed/"
        return if (withSlash.endsWith("/api/v1/")) withSlash else "${withSlash}api/v1/"
    }
}
