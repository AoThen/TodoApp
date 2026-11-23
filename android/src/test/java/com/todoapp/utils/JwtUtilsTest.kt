package com.todoapp.utils

import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

// JwtUtils 依赖 org.json.JSONObject，JVM 单测需 Robolectric 提供真实实现（与项目现有测试保持一致）
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class JwtUtilsTest {

    private fun jwt(payloadJson: String): String {
        // 真实 JWT payload 是 Base64URL 编码（无 padding），与 JwtUtils 的解码方式保持一致
        val payload = java.util.Base64.getUrlEncoder().encodeToString(payloadJson.toByteArray())
        return "header.$payload.signature"
    }

    @Test
    fun `有效 access token 返回 Valid`() {
        val token = jwt("""{"user_id":"42","token_type":"access","exp":${System.currentTimeMillis() / 1000 + 3600}}""")
        assertTrue(JwtUtils.validateAccessToken(token) is JwtUtils.TokenValidation.Valid)
    }

    @Test
    fun `过期 access token 返回 Expired`() {
        val token = jwt("""{"user_id":"42","token_type":"access","exp":${System.currentTimeMillis() / 1000 - 10}}""")
        assertTrue(JwtUtils.validateAccessToken(token) is JwtUtils.TokenValidation.Expired)
    }

    @Test
    fun `格式非法 token 返回 InvalidFormat`() {
        assertTrue(JwtUtils.validateAccessToken("not-a-jwt") is JwtUtils.TokenValidation.InvalidFormat)
    }

    @Test
    fun `类型非 access 返回 InvalidType`() {
        val token = jwt("""{"user_id":"42","token_type":"refresh","exp":${System.currentTimeMillis() / 1000 + 3600}}""")
        assertTrue(JwtUtils.validateAccessToken(token) is JwtUtils.TokenValidation.InvalidType)
    }
}
