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
    private const val KEY_VERSION_KEY = "key_version"
    private const val CURRENT_KEY_VERSION = 2
    private const val IV_TRANSFORMATION = "AES/GCM/NoPadding"

    private var keyStore: KeyStore? = null

    fun init() {
        try {
            keyStore = KeyStore.getInstance(KEYSTORE_PROVIDER).apply {
                load(null)
            }
        } catch (e: Exception) {
            keyStore = null
        }
    }

    private fun ensureInitialized() {
        if (keyStore == null) {
            init()
        }
    }

    fun hasKey(context: Context): Boolean {
        return try {
            val prefs = context.getSharedPreferences(ENCRYPTION_PREFERENCE, Context.MODE_PRIVATE)
            prefs.getInt(KEY_VERSION_KEY, 0) == CURRENT_KEY_VERSION &&
                !prefs.getString(KEY_KEY, null).isNullOrEmpty()
        } catch (e: Exception) {
            false
        }
    }

    /**
     * 保存配对下发的每设备密钥与服务器地址（仅写入 Keystore 加密后的密文）。
     * 返回 false 表示本机安全存储不可用——调用方应视为配对失败（fail-closed，不再明文兜底）。
     */
    fun saveKey(context: Context, key: String, serverUrl: String): Boolean {
        return try {
            ensureInitialized()
            val prefs = context.getSharedPreferences(ENCRYPTION_PREFERENCE, Context.MODE_PRIVATE)

            if (keyStore != null && !keyExistsInKeystore()) {
                createKeyInKeystore()
            }

            if (keyStore == null || !keyExistsInKeystore()) {
                android.util.Log.w("KeyStorage", "Keystore 不可用，拒绝明文保存设备密钥（fail-closed）")
                return false
            }

            val encryptedKey = encryptWithKeystore(key)
            val encryptedServerUrl = encryptWithKeystore(serverUrl)

            prefs.edit()
                .putString(KEY_KEY, encryptedKey)
                .putString(SERVER_URL_KEY, encryptedServerUrl)
                .putInt(KEY_VERSION_KEY, CURRENT_KEY_VERSION)
                .apply()
            true
        } catch (e: Exception) {
            android.util.Log.w("KeyStorage", "保存设备密钥失败（fail-closed）", e)
            false
        }
    }

    fun getKey(context: Context): String? {
        return try {
            ensureInitialized()
            val prefs = context.getSharedPreferences(ENCRYPTION_PREFERENCE, Context.MODE_PRIVATE)
            if (prefs.getInt(KEY_VERSION_KEY, 0) != CURRENT_KEY_VERSION) {
                return null
            }
            val encryptedKey = prefs.getString(KEY_KEY, null) ?: return null
            if (keyStore == null || !keyExistsInKeystore()) {
                return null
            }
            decryptWithKeystore(encryptedKey)
        } catch (e: Exception) {
            null
        }
    }

    fun getServerUrl(context: Context): String? {
        return try {
            ensureInitialized()
            val prefs = context.getSharedPreferences(ENCRYPTION_PREFERENCE, Context.MODE_PRIVATE)
            if (prefs.getInt(KEY_VERSION_KEY, 0) != CURRENT_KEY_VERSION) {
                return null
            }
            val encryptedUrl = prefs.getString(SERVER_URL_KEY, null) ?: return null
            if (keyStore == null || !keyExistsInKeystore()) {
                return null
            }
            decryptWithKeystore(encryptedUrl)
        } catch (e: Exception) {
            null
        }
    }

    fun clearKey(context: Context) {
        try {
            val prefs = context.getSharedPreferences(ENCRYPTION_PREFERENCE, Context.MODE_PRIVATE)
            prefs.edit()
                .remove(KEY_KEY)
                .remove(SERVER_URL_KEY)
                .remove(KEY_VERSION_KEY)
                .apply()

            deleteKeyFromKeystore()
        } catch (e: Exception) {
            // 清理失败不影响调用方语义：本地 prefs 已清除，Keystore 残留项下次配对时覆盖
            android.util.Log.d("KeyStorage", "clearKey: keystore cleanup skipped", e)
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
            android.util.Log.w("KeyStorage", "createKeyInKeystore failed", e)
        }
    }

    private fun deleteKeyFromKeystore() {
        try {
            keyStore?.deleteEntry(KEY_ALIAS)
        } catch (e: Exception) {
            android.util.Log.d("KeyStorage", "deleteKeyFromKeystore failed", e)
        }
    }

    private fun encryptWithKeystore(plaintext: String): String {
        val secretKey = keyStore?.getKey(KEY_ALIAS, null) as? SecretKey
            ?: throw IllegalStateException("Keystore 密钥不可用")
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
        val secretKey = keyStore?.getKey(KEY_ALIAS, null) as? SecretKey
            ?: throw IllegalStateException("Keystore 密钥不可用")

        val iv = combined.copyOfRange(0, 12)
        val ciphertext = combined.copyOfRange(12, combined.size)

        val cipher = Cipher.getInstance(IV_TRANSFORMATION)
        val spec = GCMParameterSpec(128, iv)
        cipher.init(Cipher.DECRYPT_MODE, secretKey, spec)

        val plaintext = cipher.doFinal(ciphertext)
        return String(plaintext, Charsets.UTF_8)
    }
}
