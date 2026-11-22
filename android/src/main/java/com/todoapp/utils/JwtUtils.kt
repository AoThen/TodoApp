package com.todoapp.utils

import org.json.JSONObject
import java.util.Base64

object JwtUtils {
    private const val UNKNOWN_USER = "unknown-user"

    /**
     * 从 JWT payload 中解析 user_id。
     * JWT payload 是 Base64URL 编码（含 -/_ 且无 padding），
     * 必须使用 URL 解码器；标准 Base64 解码会抛 IllegalArgumentException。
     */
    fun parseUserId(token: String): String {
        return try {
            val payload = decodePayload(token) ?: return UNKNOWN_USER
            val json = JSONObject(payload)
            json.optString("user_id", json.optString("sub", UNKNOWN_USER))
        } catch (e: Exception) {
            UNKNOWN_USER
        }
    }

    /**
     * 校验 JWT 是否为未过期的 access token。
     * 不验证签名（签名由服务端 authMiddleware 校验），仅做格式与过期预检。
     */
    fun validateAccessToken(token: String): TokenValidation {
        return try {
            val parts = token.split(".")
            if (parts.size != 3) {
                return TokenValidation.InvalidFormat
            }

            val payload = decodePayload(token) ?: return TokenValidation.InvalidFormat
            val json = JSONObject(payload)

            val exp = json.optLong("exp", 0)
            if (exp > 0 && exp < System.currentTimeMillis() / 1000) {
                return TokenValidation.Expired
            }

            val tokenType = json.optString("token_type", "")
            if (tokenType.isNotEmpty() && tokenType != "access") {
                return TokenValidation.InvalidType
            }

            TokenValidation.Valid
        } catch (e: Exception) {
            TokenValidation.Error
        }
    }

    private fun decodePayload(token: String): String? {
        val parts = token.split(".")
        if (parts.size != 3) return null
        return try {
            Base64.getUrlDecoder().decode(parts[1]).toString(Charsets.UTF_8)
        } catch (e: IllegalArgumentException) {
            null
        }
    }

    sealed class TokenValidation {
        object Valid : TokenValidation()
        object Expired : TokenValidation()
        object InvalidFormat : TokenValidation()
        object InvalidType : TokenValidation()
        object Error : TokenValidation()
    }
}
