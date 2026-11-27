package com.todoapp.data.crypto

import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/**
 * KeyStorage 单元测试。
 *
 * JVM/Robolectric 环境没有 AndroidKeyStore provider，init() 会降级为 keyStore == null。
 * 按 fail-closed 设计，此时 saveKey 返回 false 且不落盘（不再明文兜底），getKey/getServerUrl 返回 null。
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class KeyStorageTest {

    private val context = ApplicationProvider.getApplicationContext<android.content.Context>()
    private val keyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
    private val serverUrl = "http://10.0.2.2:8080/api/v1/"

    @Before
    fun setUp() {
        KeyStorage.init()
        KeyStorage.clearKey(context)
    }

    @Test
    fun `无密钥时 hasKey 为 false`() {
        assertFalse(KeyStorage.hasKey(context))
    }

    @Test
    fun `Keystore 不可用时 saveKey 失败且不落盘`() {
        assertFalse(KeyStorage.saveKey(context, keyHex, serverUrl))
        assertFalse(KeyStorage.hasKey(context))
        assertNull(KeyStorage.getKey(context))
        assertNull(KeyStorage.getServerUrl(context))
    }

    @Test
    fun `未保存时 getKey 与 getServerUrl 返回 null`() {
        assertNull(KeyStorage.getKey(context))
        assertNull(KeyStorage.getServerUrl(context))
    }

    @Test
    fun `clearKey 后状态保持未配对`() {
        KeyStorage.saveKey(context, keyHex, serverUrl)
        KeyStorage.clearKey(context)
        assertFalse(KeyStorage.hasKey(context))
        assertNull(KeyStorage.getKey(context))
        assertNull(KeyStorage.getServerUrl(context))
    }

    @Test
    fun `hasValidPairing 在无有效密钥时为 false`() {
        KeyStorage.saveKey(context, keyHex, serverUrl)
        assertFalse(KeyStorage.hasValidPairing(context))
    }

    @Test
    fun `旧版本数据不满足版本要求时视为未配对`() {
        val prefs = context.getSharedPreferences("todoapp_encryption_prefs", android.content.Context.MODE_PRIVATE)
        prefs.edit()
            .putString("encryption_key", keyHex)
            .putString("server_url", serverUrl)
            .putInt("key_version", 1)
            .apply()
        assertFalse(KeyStorage.hasValidPairing(context))
        assertNull(KeyStorage.getKey(context))
    }

    @Test
    fun `重复 saveKey 不抛异常`() {
        repeat(3) {
            assertFalse(KeyStorage.saveKey(context, keyHex, serverUrl))
        }
    }
}
