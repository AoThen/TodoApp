package com.todoapp.data.crypto

import android.util.Base64
import java.io.ByteArrayOutputStream
import java.security.KeyFactory
import java.security.KeyPair
import java.security.KeyPairGenerator
import java.security.MessageDigest
import java.security.PrivateKey
import java.security.spec.ECGenParameterSpec
import java.security.spec.X509EncodedKeySpec
import javax.crypto.Cipher
import javax.crypto.KeyAgreement
import javax.crypto.Mac
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

/**
 * 每设备密钥的 ECDH 领取（与 Go 端 WrapDeviceKey、Web 端 unwrapDeviceKey 严格对齐）：
 * P-256 + HKDF-SHA256 + AES-256-GCM，AAD = clientSPKI || serverSPKI || deviceId || pairingKey。
 */
object EcdhKeyWrap {
    private const val INFO_PREFIX = "todoapp/device-key-v2"
    private const val CURVE = "secp256r1"

    data class ClientKeyPair(
        val privateKey: PrivateKey,
        val publicKeySpki: ByteArray,
        val publicKeySpkiB64: String
    )

    /** 生成一次性 P-256 客户端密钥对（私钥仅存在于内存，用于本次解包） */
    fun generateClientKeyPair(): ClientKeyPair {
        val generator = KeyPairGenerator.getInstance("EC")
        generator.initialize(ECGenParameterSpec(CURVE))
        val keyPair: KeyPair = generator.generateKeyPair()
        val spki = keyPair.public.encoded
        return ClientKeyPair(
            privateKey = keyPair.private,
            publicKeySpki = spki,
            publicKeySpkiB64 = Base64.encodeToString(spki, Base64.NO_WRAP)
        )
    }

    /** 解包服务端下发的设备密钥，返回 64 字符 hex */
    fun unwrapDeviceKey(
        clientPrivateKey: PrivateKey,
        clientPublicSpki: ByteArray,
        serverPublicKeyB64: String,
        wrappedKeyB64: String,
        nonceB64: String,
        deviceId: String,
        pairingKey: String
    ): String {
        val serverSpki = Base64.decode(serverPublicKeyB64, Base64.NO_WRAP)
        val serverPublicKey = KeyFactory.getInstance("EC")
            .generatePublic(X509EncodedKeySpec(serverSpki))

        val agreement = KeyAgreement.getInstance("ECDH")
        agreement.init(clientPrivateKey)
        agreement.doPhase(serverPublicKey, true)
        val sharedSecret = agreement.generateSecret()

        val salt = sha256(concat(clientPublicSpki, serverSpki))
        val info = "$INFO_PREFIX|$deviceId".toByteArray(Charsets.UTF_8)
        val kek = hkdfSha256(sharedSecret, salt, info, 32)

        val aad = concat(
            clientPublicSpki,
            serverSpki,
            deviceId.toByteArray(Charsets.UTF_8),
            pairingKey.toByteArray(Charsets.UTF_8)
        )

        val nonce = Base64.decode(nonceB64, Base64.NO_WRAP)
        val ciphertext = Base64.decode(wrappedKeyB64, Base64.NO_WRAP)

        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, SecretKeySpec(kek, "AES"), GCMParameterSpec(128, nonce))
        cipher.updateAAD(aad)
        val plaintext = cipher.doFinal(ciphertext)

        if (plaintext.size != 32) {
            throw IllegalStateException("设备密钥长度无效")
        }
        return plaintext.joinToString("") { "%02x".format(it) }
    }

    private fun sha256(data: ByteArray): ByteArray {
        return MessageDigest.getInstance("SHA-256").digest(data)
    }

    private fun concat(vararg parts: ByteArray): ByteArray {
        val out = ByteArrayOutputStream()
        for (part in parts) {
            out.write(part)
        }
        return out.toByteArray()
    }

    /** RFC 5869 HKDF-SHA256 */
    private fun hkdfSha256(ikm: ByteArray, salt: ByteArray, info: ByteArray, length: Int): ByteArray {
        val mac = Mac.getInstance("HmacSHA256")
        val actualSalt = if (salt.isEmpty()) ByteArray(32) else salt
        mac.init(SecretKeySpec(actualSalt, "HmacSHA256"))
        val prk = mac.doFinal(ikm)

        mac.init(SecretKeySpec(prk, "HmacSHA256"))
        val okm = ByteArrayOutputStream()
        var previous = ByteArray(0)
        var counter = 1
        while (okm.size() < length) {
            mac.reset()
            mac.update(previous)
            mac.update(info)
            mac.update(counter.toByte())
            previous = mac.doFinal()
            okm.write(previous)
            counter++
        }
        return okm.toByteArray().copyOf(length)
    }
}
