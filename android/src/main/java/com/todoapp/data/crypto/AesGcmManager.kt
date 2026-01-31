package com.todoapp.data.crypto

import android.util.Base64
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

object AesGcmManager {
    private const val ALGORITHM = "AES/GCM/NoPadding"
    private const val KEY_SIZE = 256
    private const val GCM_IV_LENGTH = 12
    private const val GCM_TAG_LENGTH = 128

    fun encrypt(plaintext: String, keyHex: String): String {
        val key = hexStringToByteArray(keyHex)
        val secretKey = SecretKeySpec(key, "AES")

        val iv = ByteArray(GCM_IV_LENGTH)
        SecureRandom().nextBytes(iv)

        val cipher = Cipher.getInstance(ALGORITHM)
        val gcmSpec = GCMParameterSpec(GCM_TAG_LENGTH, iv)
        cipher.init(Cipher.ENCRYPT_MODE, secretKey, gcmSpec)

        val ciphertext = cipher.doFinal(plaintext.toByteArray(Charsets.UTF_8))

        val combined = ByteArray(iv.size + ciphertext.size)
        System.arraycopy(iv, 0, combined, 0, iv.size)
        System.arraycopy(ciphertext, 0, combined, iv.size, ciphertext.size)

        return Base64.encodeToString(combined, Base64.NO_WRAP)
    }

    fun decrypt(encryptedBase64: String, keyHex: String): String {
        val key = hexStringToByteArray(keyHex)
        val secretKey = SecretKeySpec(key, "AES")

        val combined = Base64.decode(encryptedBase64, Base64.NO_WRAP)

        val iv = combined.copyOfRange(0, GCM_IV_LENGTH)
        val ciphertext = combined.copyOfRange(GCM_IV_LENGTH, combined.size)

        val cipher = Cipher.getInstance(ALGORITHM)
        val gcmSpec = GCMParameterSpec(GCM_TAG_LENGTH, iv)
        cipher.init(Cipher.DECRYPT_MODE, secretKey, gcmSpec)

        val plaintext = cipher.doFinal(ciphertext)
        return String(plaintext, Charsets.UTF_8)
    }

    fun encryptBytes(data: ByteArray, keyHex: String): ByteArray {
        val key = hexStringToByteArray(keyHex)
        val secretKey = SecretKeySpec(key, "AES")

        val iv = ByteArray(GCM_IV_LENGTH)
        SecureRandom().nextBytes(iv)

        val cipher = Cipher.getInstance(ALGORITHM)
        val gcmSpec = GCMParameterSpec(GCM_TAG_LENGTH, iv)
        cipher.init(Cipher.ENCRYPT_MODE, secretKey, gcmSpec)

        val ciphertext = cipher.doFinal(data)

        val combined = ByteArray(iv.size + ciphertext.size)
        System.arraycopy(iv, 0, combined, 0, iv.size)
        System.arraycopy(ciphertext, 0, combined, iv.size, ciphertext.size)

        return combined
    }

    fun decryptBytes(encryptedData: ByteArray, keyHex: String): ByteArray {
        val key = hexStringToByteArray(keyHex)
        val secretKey = SecretKeySpec(key, "AES")

        val iv = encryptedData.copyOfRange(0, GCM_IV_LENGTH)
        val ciphertext = encryptedData.copyOfRange(GCM_IV_LENGTH, encryptedData.size)

        val cipher = Cipher.getInstance(ALGORITHM)
        val gcmSpec = GCMParameterSpec(GCM_TAG_LENGTH, iv)
        cipher.init(Cipher.DECRYPT_MODE, secretKey, gcmSpec)

        return cipher.doFinal(ciphertext)
    }

    fun generateKey(): String {
        val bytes = ByteArray(32)
        SecureRandom().nextBytes(bytes)
        return bytes.joinToString("") { "%02x".format(it) }
    }

    fun isValidKey(keyHex: String): Boolean {
        return try {
            val key = hexStringToByteArray(keyHex)
            key.size == 32
        } catch (e: Exception) {
            false
        }
    }

    private fun hexStringToByteArray(s: String): ByteArray {
        val len = s.length
        val data = ByteArray(len / 2)
        var i = 0
        while (i < len) {
            data[i / 2] = ((Character.digit(s[i], 16) shl 4) + Character.digit(s[i + 1], 16)).toByte()
            i += 2
        }
        return data
    }
}
