package com.todoapp.data.crypto

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

object KeyStorage {
    private const val KEYSTORE_PROVIDER = "AndroidKeyStore"
    private const val KEY_ALIAS = "TodoAppEncryptionKey"
    private const val ENCRYPTION_PREFERENCE = "todoapp_encryption_prefs"
    private const val KEY_KEY = "encryption_key"
    private const val SERVER_URL_KEY = "server_url"
    private const val IV_TRANSFORMATION = "AES/GCM/NoPadding"

    private var keyStore: KeyStore? = null

    fun init(context: Context) {
        try {
            keyStore = KeyStore.getInstance(KEYSTORE_PROVIDER).apply {
                load(null)
            }
        } catch (e: Exception) {
            keyStore = null
        }
    }

    fun hasKey(context: Context): Boolean {
        return try {
            val prefs = context.getSharedPreferences(ENCRYPTION_PREFERENCE, Context.MODE_PRIVATE)
            val encryptedKey = prefs.getString(KEY_KEY, null)
            encryptedKey != null
        } catch (e: Exception) {
            false
        }
    }

    fun saveKey(context: Context, key: String, serverUrl: String) {
        try {
            val prefs = context.getSharedPreferences(ENCRYPTION_PREFERENCE, Context.MODE_PRIVATE)

            if (keyStore != null && !keyExistsInKeystore()) {
                createKeyInKeystore()
            }

            val encryptedKey = if (keyStore != null && keyExistsInKeystore()) {
                encryptWithKeystore(key)
            } else {
                key
            }

            val encryptedServerUrl = if (keyStore != null && keyExistsInKeystore()) {
                encryptWithKeystore(serverUrl)
            } else {
                serverUrl
            }

            prefs.edit()
                .putString(KEY_KEY, encryptedKey)
                .putString(SERVER_URL_KEY, encryptedServerUrl)
                .apply()
        } catch (e: Exception) {
            val prefs = context.getSharedPreferences(ENCRYPTION_PREFERENCE, Context.MODE_PRIVATE)
            prefs.edit()
                .putString(KEY_KEY, key)
                .putString(SERVER_URL_KEY, serverUrl)
                .apply()
        }
    }

    fun getKey(context: Context): String? {
        try {
            val prefs = context.getSharedPreferences(ENCRYPTION_PREFERENCE, Context.MODE_PRIVATE)
            val encryptedKey = prefs.getString(KEY_KEY, null) ?: return null

            return if (keyStore != null && keyExistsInKeystore() && !isHexKey(encryptedKey)) {
                decryptWithKeystore(encryptedKey)
            } else {
                encryptedKey
            }
        } catch (e: Exception) {
            return null
        }
    }

    fun getServerUrl(context: Context): String? {
        try {
            val prefs = context.getSharedPreferences(ENCRYPTION_PREFERENCE, Context.MODE_PRIVATE)
            val encryptedUrl = prefs.getString(SERVER_URL_KEY, null) ?: return null

            return if (keyStore != null && keyExistsInKeystore() && !isHttpUrl(encryptedUrl)) {
                decryptWithKeystore(encryptedUrl)
            } else {
                encryptedUrl
            }
        } catch (e: Exception) {
            return null
        }
    }

    fun clearKey(context: Context) {
        try {
            val prefs = context.getSharedPreferences(ENCRYPTION_PREFERENCE, Context.MODE_PRIVATE)
            prefs.edit()
                .remove(KEY_KEY)
                .remove(SERVER_URL_KEY)
                .apply()

            deleteKeyFromKeystore()
        } catch (e: Exception) {
        }
    }

    fun hasValidPairing(context: Context): Boolean {
        val key = getKey(context)
        val serverUrl = getServerUrl(context)
        return !key.isNullOrEmpty() && !serverUrl.isNullOrEmpty()
    }

    private fun keyExistsInKeystore(): Boolean {
        return try {
            keyStore?.containsAlias(KEY_ALIAS) == true
        } catch (e: Exception) {
            false
        }
    }

    private fun createKeyInKeystore() {
        try {
            val keyGenerator = KeyGenerator.getInstance(
                KeyProperties.KEY_ALGORITHM_AES,
                KEYSTORE_PROVIDER
            )

            val keyGenSpec = KeyGenParameterSpec.Builder(
                KEY_ALIAS,
                KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT
            )
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .setUserAuthenticationRequired(false)
                .build()

            keyGenerator.init(keyGenSpec)
            keyGenerator.generateKey()
        } catch (e: Exception) {
        }
    }

    private fun deleteKeyFromKeystore() {
        try {
            keyStore?.deleteEntry(KEY_ALIAS)
        } catch (e: Exception) {
        }
    }

    private fun encryptWithKeystore(plaintext: String): String {
        val secretKey = keyStore?.getKey(KEY_ALIAS, null) as SecretKey
        val cipher = Cipher.getInstance(IV_TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, secretKey)

        val iv = cipher.iv
        val ciphertext = cipher.doFinal(plaintext.toByteArray(Charsets.UTF_8))

        val combined = ByteArray(iv.size + ciphertext.size)
        System.arraycopy(iv, 0, combined, 0, iv.size)
        System.arraycopy(ciphertext, 0, combined, iv.size, ciphertext.size)

        return android.util.Base64.encodeToString(combined, android.util.Base64.NO_WRAP)
    }

    private fun decryptWithKeystore(encryptedBase64: String): String {
        val combined = android.util.Base64.decode(encryptedBase64, android.util.Base64.NO_WRAP)
        val secretKey = keyStore?.getKey(KEY_ALIAS, null) as SecretKey

        val iv = combined.copyOfRange(0, 12)
        val ciphertext = combined.copyOfRange(12, combined.size)

        val cipher = Cipher.getInstance(IV_TRANSFORMATION)
        val spec = GCMParameterSpec(128, iv)
        cipher.init(Cipher.DECRYPT_MODE, secretKey, spec)

        val plaintext = cipher.doFinal(ciphertext)
        return String(plaintext, Charsets.UTF_8)
    }

    private fun isHexKey(s: String): Boolean {
        return s.matches(Regex("[0-9a-fA-F]{64}"))
    }

    private fun isHttpUrl(s: String): Boolean {
        return s.startsWith("http://") || s.startsWith("https://")
    }
}
   