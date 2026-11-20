package com.todoapp.data.crypto

import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.util.Base64
import javax.crypto.AEADBadTagException

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class AesGcmManagerTest {

    private val keyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

    @Test
    fun `加解密往返一致（含中文与emoji）`() {
        val plaintext = "完成 TODO 测试 🚀 中文内容"
        val encrypted = AesGcmManager.encrypt(plaintext, keyHex)
        val decrypted = AesGcmManager.decrypt(encrypted, keyHex)
        assertEquals(plaintext, decrypted)
    }

    @Test
    fun `两次加密同一明文产生不同密文（随机IV）`() {
        val plaintext = "same message"
        val first = AesGcmManager.encrypt(plaintext, keyHex)
        val second = AesGcmManager.encrypt(plaintext, keyHex)
        assertNotEquals(first, second)
        assertEquals(plaintext, AesGcmManager.decrypt(first, keyHex))
        assertEquals(plaintext, AesGcmManager.decrypt(second, keyHex))
    }

    @Test
    fun `密文格式为Base64且前12字节为IV`() {
        val plaintext = "格式校验"
        val encrypted = AesGcmManager.encrypt(plaintext, keyHex)
        val combined = Base64.getDecoder().decode(encrypted)

        // IV(12) + 明文长度 + GCM Tag(16)
        val expectedLen = 12 + plaintext.toByteArray(Charsets.UTF_8).size + 16
        assertEquals(expectedLen, combined.size)
    }

    @Test(expected = AEADBadTagException::class)
    fun `篡改密文后解密失败`() {
        val encrypted = AesGcmManager.encrypt("原始内容", keyHex)
        val combined = Base64.getDecoder().decode(encrypted)
        combined[combined.size - 1] = (combined[combined.size - 1].toInt() xor 0x01).toByte()
        val tampered = Base64.getEncoder().encodeToString(combined)
        AesGcmManager.decrypt(tampered, keyHex)
    }

    @Test(expected = AEADBadTagException::class)
    fun `错误密钥解密失败`() {
        val encrypted = AesGcmManager.encrypt("原始内容", keyHex)
        val wrongKey = "f".repeat(64)
        AesGcmManager.decrypt(encrypted, wrongKey)
    }

    @Test
    fun `字节数组加解密往返一致`() {
        val data = "二进制内容 bytes-01".toByteArray(Charsets.UTF_8)
        val encrypted = AesGcmManager.encryptBytes(data, keyHex)
        assertEquals(data.size + 12 + 16, encrypted.size)
        val decrypted = AesGcmManager.decryptBytes(encrypted, keyHex)
        assertTrue(data.contentEquals(decrypted))
    }

    @Test
    fun `generateKey 生成64位hex且随机`() {
        val first = AesGcmManager.generateKey()
        val second = AesGcmManager.generateKey()
        assertEquals(64, first.length)
        assertTrue(first.matches(Regex("^[0-9a-f]{64}$")))
        assertNotEquals(first, second)
        assertTrue(AesGcmManager.isValidKey(first))
    }

    @Test
    fun `isValidKey 校验长度与hex格式`() {
        assertTrue(AesGcmManager.isValidKey(keyHex))
        assertFalse("长度不足", AesGcmManager.isValidKey("ab".repeat(16)))
        assertFalse("非hex字符", AesGcmManager.isValidKey("z".repeat(64)))
        assertFalse("空串", AesGcmManager.isValidKey(""))
    }
}
