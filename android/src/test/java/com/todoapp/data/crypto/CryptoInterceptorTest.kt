package com.todoapp.data.crypto

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import io.mockk.CapturingSlot
import io.mockk.every
import io.mockk.mockk
import io.mockk.mockkObject
import io.mockk.unmockkAll
import io.mockk.verify
import okhttp3.Interceptor
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Protocol
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import okio.Buffer

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class CryptoInterceptorTest {

    private val context = ApplicationProvider.getApplicationContext<Context>()
    private val keyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

    private lateinit var interceptor: CryptoInterceptor

    @Before
    fun setUp() {
        mockkObject(KeyStorage)
        every { KeyStorage.getKey(any()) } returns keyHex
        interceptor = CryptoInterceptor.getInstance(context)
        interceptor.refreshKey()
    }

    @After
    fun tearDown() {
        unmockkAll()
    }

    private fun buildResponse(request: Request, body: String): Response {
        return Response.Builder()
            .protocol(Protocol.HTTP_1_1)
            .code(200)
            .message("OK")
            .request(request)
            .body(body.toResponseBody("application/json".toMediaType()))
            .build()
    }

    private fun buildEncryptedResponse(request: Request, plaintext: String, keyHex: String): Response {
        // 服务端下发的是加密后的原始字节（IV+密文），而非 Base64 文本
        val cipherBytes = AesGcmManager.encryptBytes(plaintext.toByteArray(Charsets.UTF_8), keyHex)
        return Response.Builder()
            .protocol(Protocol.HTTP_1_1)
            .code(200)
            .message("OK")
            .request(request)
            .header("X-Encrypted", "true")
            .body(cipherBytes.toResponseBody("application/octet-stream".toMediaType()))
            .build()
    }

    private fun mockChain(
        request: Request,
        responseFactory: (Request) -> Response = { buildResponse(it, "{}") }
    ): Interceptor.Chain {
        val chain = mockk<Interceptor.Chain>()
        every { chain.request() } returns request
        every { chain.proceed(any()) } answers { responseFactory(firstArg()) }
        return chain
    }

    /** 执行拦截并返回实际传给 chain.proceed 的请求 */
    private fun proceedRequest(chain: Interceptor.Chain): Request {
        interceptor.intercept(chain)
        val slot = CapturingSlot<Request>()
        verify { chain.proceed(capture(slot)) }
        return slot.captured
    }

    private fun jsonRequest(url: String, method: String, body: String? = null): Request {
        val builder = Request.Builder().url(url)
        when (method) {
            "GET" -> builder.get()
            "DELETE" -> builder.delete()
            else -> builder.method(
                method,
                (body ?: "").toRequestBody("application/json".toMediaType())
            )
        }
        return builder.build()
    }

    @Test
    fun `无密钥时请求透传不加密`() {
        every { KeyStorage.getKey(any()) } returns null
        interceptor.refreshKey()

        val request = jsonRequest("http://10.0.2.2:8080/api/v1/tasks", "POST", """{"title":"t"}""")
        val sent = proceedRequest(mockChain(request))

        assertNull(sent.header("X-Encrypted"))
        assertNull(sent.header("X-Accept-Encrypted"))
    }

    @Test
    fun `POST tasks 请求体被加密并携带加密头`() {
        val original = """{"title":"写测试","priority":"high"}"""
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/tasks", "POST", original)
        val sent = proceedRequest(mockChain(request))

        assertEquals("true", sent.header("X-Encrypted"))
        assertEquals("true", sent.header("X-Accept-Encrypted"))
        val body = sent.body
        assertNotNull("请求体不应为空", body)
        val contentType = body?.contentType()?.toString()
        assertTrue(
            "实际 Content-Type: $contentType",
            contentType?.contains("octet-stream") == true
        )

        val sentBytes = Buffer().also { body!!.writeTo(it) }.readByteArray()
        val decrypted = String(AesGcmManager.decryptBytes(sentBytes, keyHex), Charsets.UTF_8)
        assertEquals(original, decrypted)
    }

    @Test
    fun `GET tasks 列表不加密`() {
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/tasks", "GET")
        val sent = proceedRequest(mockChain(request))
        assertNull(sent.header("X-Encrypted"))
    }

    @Test
    fun `单个任务 GET 不加密`() {
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/tasks/42", "GET")
        val sent = proceedRequest(mockChain(request))
        assertNull(sent.header("X-Encrypted"))
    }

    @Test
    fun `单个任务 DELETE 不加密`() {
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/tasks/42", "DELETE")
        val sent = proceedRequest(mockChain(request))
        assertNull(sent.header("X-Encrypted"))
    }

    @Test
    fun `auth 路径不加密`() {
        val request = jsonRequest(
            "http://10.0.2.2:8080/api/v1/auth/login",
            "POST",
            """{"email":"a@b.c","password":"p"}"""
        )
        val sent = proceedRequest(mockChain(request))
        assertNull(sent.header("X-Encrypted"))
    }

    @Test
    fun `admin 路径不加密`() {
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/admin/users", "GET")
        val sent = proceedRequest(mockChain(request))
        assertNull(sent.header("X-Encrypted"))
    }

    @Test
    fun `health 路径不加密`() {
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/health", "GET")
        val sent = proceedRequest(mockChain(request))
        assertNull(sent.header("X-Encrypted"))
    }

    @Test
    fun `sync 路径需要加密`() {
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/sync", "POST", """{"deltas":[]}""")
        val sent = proceedRequest(mockChain(request))
        assertEquals("true", sent.header("X-Encrypted"))
    }

    @Test
    fun `已带 X-Encrypted 头的请求不重复加密`() {
        val request = Request.Builder()
            .url("http://10.0.2.2:8080/api/v1/tasks")
            .header("X-Encrypted", "true")
            .post("""{"title":"t"}""".toRequestBody("application/json".toMediaType()))
            .build()
        val sent = proceedRequest(mockChain(request))

        // 不应再次覆盖加密流程：body 仍是原始 JSON 而非密文
        val sentBytes = Buffer().also { sent.body!!.writeTo(it) }.readByteArray()
        assertEquals("""{"title":"t"}""", String(sentBytes, Charsets.UTF_8))
    }

    @Test
    fun `加密响应体被自动解密为明文 JSON`() {
        val secret = """{"success":true,"data":{"id":1}}"""

        // POST /sync 走加密请求路径，服务端返回加密响应时应被自动解密
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/sync", "POST", """{"deltas":[]}""")
        val chain = mockChain(request) { buildEncryptedResponse(it, secret, keyHex) }
        val response = interceptor.intercept(chain)

        assertEquals(secret, response.body?.string())
        assertNull(response.header("X-Encrypted"))
    }

    @Test
    fun `未加密响应体原样返回`() {
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/sync", "POST", """{"deltas":[]}""")
        val chain = mockChain(request) { buildResponse(it, """{"ok":true}""") }
        val response = interceptor.intercept(chain)

        assertEquals("""{"ok":true}""", response.body?.string())
    }

    @Test
    fun `空请求体不触发加密直接透传`() {
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/tasks", "POST", "")
        val sent = proceedRequest(mockChain(request))
        assertNull(sent.header("X-Encrypted"))
    }
}
