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
}
