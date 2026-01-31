package com.todoapp.data.crypto

import android.content.Context
import okhttp3.Interceptor
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Protocol
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import okhttp3.ResponseBody.Companion.toResponseBody
import okhttp3.logging.HttpLoggingInterceptor
import java.util.concurrent.TimeUnit

class CryptoInterceptor private constructor(context: Context) : Interceptor {

    private val keyStorage = KeyStorage
    private var encryptionKey: String? = null
    private val context: Context = context.applicationContext

    init {
        keyStorage.init(context)
        refreshKey()
    }

    fun refreshKey() {
        encryptionKey = keyStorage.getKey(context)
    }

    override fun intercept(chain: Interceptor.Chain): Response {
        val originalRequest = chain.request()
        val key = encryptionKey

        if (key.isNullOrEmpty()) {
            return chain.proceed(originalRequest)
        }

        val path = originalRequest.url.encodedPath
        val method = originalRequest.method

        val shouldEncrypt = shouldEncryptPath(path, method)

        if (shouldEncrypt && originalRequest.header("X-Encrypted") == null) {
            val originalBody = originalRequest.body ?: return chain.proceed(originalRequest)

            val originalBodyBytes = buffer(originalBody)
            val bodyString = originalBodyBytes.clone().toString(Charsets.UTF_8)

            if (bodyString.isNotEmpty()) {
                try {
                    val encryptedBody = AesGcmManager.encrypt(bodyString, key)
                    val newBody = encryptedBody.toRequestBody("application/octet-stream".toMediaType())

                    val newRequest = originalRequest.newBuilder()
                        .header("X-Encrypted", "true")
                        .header("X-Accept-Encrypted", "true")
                        .method(originalRequest.method, newBody)
                        .build()

                    return proceedWithDecryption(chain, newRequest, key)
                } catch (e: Exception) {
                    return createErrorResponse(chain, "Encryption failed: ${e.message}")
                }
            }
        }

        if (originalRequest.header("X-Accept-Encrypted") == "true") {
            return proceedWithDecryption(chain, originalRequest, key)
        }

        return chain.proceed(originalRequest)
    }

    private fun proceedWithDecryption(chain: Interceptor.Chain, request: okhttp3.Request, key: String): Response {
        val response = chain.proceed(request)

        if (response.header("X-Encrypted") != "true") {
            return response
        }

        val responseBody = response.body ?: return response
        val responseBytes = bufferResponse(responseBody)

        try {
            val decrypted = AesGcmManager.decryptBytes(responseBytes, key)
            val decryptedString = String(decrypted, Charsets.UTF_8)

            return response.newBuilder()
                .body(decryptedString.toResponseBody("application/json".toMediaType()))
                .removeHeader("X-Encrypted")
                .build()
        } catch (e: Exception) {
            return createErrorResponse(chain, "Decryption failed: ${e.message}")
        }
    }

    private fun buffer(body: okhttp3.RequestBody): ByteArray {
        val buffer = okio.Buffer()
        body.writeTo(buffer)
        return buffer.readByteArray()
    }

    private fun bufferResponse(body: okhttp3.ResponseBody): ByteArray {
        val buffer = okio.Buffer()
        body.writeTo(buffer)
        return buffer.readByteArray()
    }

        val responseBody = response.body ?: return response
        val responseBytes = buffer(responseBody)

        try {
            val decrypted = AesGcmManager.decryptBytes(responseBytes, key)
            val decryptedString = String(decrypted, Charsets.UTF_8)

            return response.newBuilder()
                .body(decryptedString.toResponseBody("application/json".toMediaType()))
                .removeHeader("X-Encrypted")
                .build()
        } catch (e: Exception) {
            return createErrorResponse(chain, "Decryption failed: ${e.message}")
        }
    }

    private fun shouldEncryptPath(path: String, method: String): Boolean {
        return when {
            path.contains("/auth/") -> false
            path.contains("/health") -> false
            path == "/api/v1/tasks" && method == "GET" -> false
            path.startsWith("/api/v1/tasks/") && method == "GET" -> false
            path.startsWith("/api/v1/tasks/") && method == "DELETE" -> false
            path.contains("/admin/") -> false
            else -> true
        }
    }

    private fun buffer(body: okhttp3.RequestBody): ByteArray {
        val buffer = okio.Buffer()
        body.writeTo(buffer)
        return buffer.readByteArray()
    }

    private fun createErrorResponse(chain: Interceptor.Chain, message: String): Response {
        return Response.Builder()
            .protocol(Protocol.HTTP_1_1)
            .code(500)
            .message("Encryption Error")
            .body("{\"error\":\"$message\"}".toResponseBody("application/json".toMediaType()))
            .request(chain.request())
            .build()
    }

    companion object {
        @Volatile
        private var instance: CryptoInterceptor? = null

        fun getInstance(context: Context): CryptoInterceptor {
            return instance ?: synchronized(this) {
                instance ?: CryptoInterceptor(context).also { instance = it }
            }
        }

        fun createOkHttpClient(context: Context): OkHttpClient {
            return OkHttpClient.Builder()
                .addInterceptor(getInstance(context))
                .addInterceptor(HttpLoggingInterceptor().apply {
                    level = if (com.todoapp.BuildConfig.DEBUG) {
                        HttpLoggingInterceptor.Level.BODY
                    } else {
                        HttpLoggingInterceptor.Level.NONE
                    }
                })
                .connectTimeout(30, TimeUnit.SECONDS)
                .readTimeout(30, TimeUnit.SECONDS)
                .writeTimeout(30, TimeUnit.SECONDS)
                .retryOnConnectionFailure(true)
                .build()
        }
    }
}
