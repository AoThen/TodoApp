package com.todoapp.data.crypto

import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/**
 * KeyStorage 单元测试。
 *
 * JVM/Robolectric 环境没有 AndroidKeyStore provider，init() 必然降级为
 * keyStore == null，因此这里的用例同时覆盖了“Keystore 不可用 → 明文兜底”
 * 这条 fallback 路径。
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class KeyStorageTest {

    private val context = ApplicationProvider.getApplicationContext<android.content.Context>()
    private val keyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
    private val serverUrl = "http://10.0.2.2:8080/api/v1/"

    @Before
    fun setUp() {
        // JVM 环境下 AndroidKeyStore 不可用，init() 走异常分支，keyStore 置空
        KeyStorage.init()
        KeyStorage.clearKey(context)
    }

    @Test
    fun `无密钥时 hasKey 为 false`() {
        assertFalse(KeyStorage.hasKey(context))
    }

    @Test
    fun `saveKey 后 hasKey 为 true`() {
        KeyStorage.saveKey(context, keyHex, serverUrl)
        assertTrue(KeyStorage.hasKey(context))
    }

    @Test
    fun `Keystore 不可用时 saveKey 明文兜底且 getKey 可还原`() {
        // JVM 环境无 AndroidKeyStore：saveKey 应返回 false（明文兜底）并可还原
        assertFalse(KeyStorage.saveKey(context, keyHex, serverUrl))
        assertEquals(keyHex, KeyStorage.getKey(context))
    }

    @Test
    fun `Keystore 不可用时 serverUrl 明文兜底且 getServerUrl 可还原`() {
        assertFalse(KeyStorage.saveKey(context, keyHex, serverUrl))
        assertEquals(serverUrl, KeyStorage.getServerUrl(context))
    }

    @Test
    fun `未保存时 getKey 与 getServerUrl 返回 null`() {
        assertNull(KeyStorage.getKey(context))
        assertNull(KeyStorage.getServerUrl(context))
    }

    @Test
    fun `覆盖保存以最新值为准`() {
        KeyStorage.saveKey(context, keyHex, serverUrl)
        val newKey = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
        KeyStorage.saveKey(context, newKey, "https://example.com/api/v1/")
        assertEquals(newKey, KeyStorage.getKey(context))
        assertEquals("https://example.com/api/v1/", KeyStorage.getServerUrl(context))
    }

    @Test
    fun `clearKey 后密钥与服务地址都被清除`() {
        KeyStorage.saveKey(context, keyHex, serverUrl)
        KeyStorage.clearKey(context)
        assertFalse(KeyStorage.hasKey(context))
        assertNull(KeyStorage.getKey(context))
        assertNull(KeyStorage.getServerUrl(context))
    }

    @Test
    fun `hasValidPairing 密钥与服务地址齐全时为 true`() {
        KeyStorage.saveKey(context, keyHex, serverUrl)
        assertTrue(KeyStorage.hasValidPairing(context))
    }

    @Test
    fun `hasValidPairing 缺少服务地址时为 false`() {
        val prefs = context.getSharedPreferences("todoapp_encryption_prefs", android.content.Context.MODE_PRIVATE)
        prefs.edit().putString("encryption_key", keyHex).apply()
        assertFalse(KeyStorage.hasValidPairing(context))
    }

    @Test
    fun `hasValidPairing 空字符串视为未配对`() {
        KeyStorage.saveKey(context, "", "")
        assertFalse(KeyStorage.hasValidPairing(context))
    }

    @Test
    fun `重复 saveKey 不抛异常且状态一致`() {
        repeat(3) { KeyStorage.saveKey(context, keyHex, serverUrl) }
        assertEquals(keyHex, KeyStorage.getKey(context))
        assertEquals(serverUrl, KeyStorage.getServerUrl(context))
    }
}
